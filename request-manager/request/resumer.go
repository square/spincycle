// Copyright 2018-2019, Square, Inc.

package request

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	serr "github.com/square/spincycle/errors"
	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/proto"
)

var (
	ErrNotUpdated      = errors.New("no row updated")
	ErrMultipleUpdated = errors.New("multiple rows updated/deleted, expected single-row update/delete")
)

type Resumer interface {
	// Suspend marks a running request as suspended and saves the corresponding
	// suspended job chain.
	Suspend(sjc proto.SuspendedJobChain) error

	// ResumeAll tries to resume all the SJCs currently stored in the database.
	ResumeAll()

	// Resume tries to resume a single SJC given its id and a connection to the
	// database. The SJC must be claimed (`rm_host` field for the SJC must be set
	// to the hostname given when creating the Resumer) before calling Resume, or
	// it will fail.
	Resume(id string) error

	// Cleanup cleans up abandoned and old SJCs. Abandoned SJCs are those that have
	// been claimed by an RM (`rm_host` field set) but have not been updated in a
	// while, meaning the RM resuming them probably crashed. These SJCs are
	// unclaimed (set `rm_host` to null) so they can be resumed in the future. Old
	// SJCs are those which haven't been resumed within the TTL provided when
	// creating the Resumer (rounded to the nearest second). They're deleted and
	// their requests' states set to FAILED.
	Cleanup()
}

// TODO(felixp): This kind of comment can probably be moved out of the code
// and into the docs once we start writing those.
// --------------------------------------------------------------------------
// Request Resumer:
//
// When a Job Runner shuts down, it suspends all the job chains it's currently
// running by sending Suspended Job Chains (SJCs), which contain all the info
// necessary to recreate the running job chain, to the Request Manager. In the RM,
// incoming SJCs are sent to the Resumer, which saves the SJC and sets the
// corresponding request's state to Suspended.
//
// When the Resumer is running, every few seconds it attempts to resume all the SJCs
// stored in its database. For each, the Resumer first "claims" the SJC -
// marking in the database that it is working on resuming this SJC, so no other
// Resumer (in another RM instance) will simultaneosly try to resume the SJC. The
// Resumer checks that the associated request's state is really Suspended, and then
// the SJC gets sent to a Job Runner. If this is successful, the request's state
// gets updated to Running and the SJC is deleted. If sending the SJC to the Job
// Runner fails (perhaps there is no Job Runner instance currently running), the
// Resumer "unclaims" the SJC, removing its claim on the SJC and allowing another
// Resumer (or the same Resumer at a later time) to retry this process.
//
// After the attempting to resume all SJCs, the Resumer does some cleanup of any
// remaining SJCs it has stored. Old SJCs, which were suspended more than an hour
// ago, are deleted and their request's states changed from Suspended to Failed.
// Abandonded SJCs, which have been claimed by another Resumer for a while but
// haven't been successfully resumed, are unclaimed - the Resumers that claimed
// these requests probably crashed midway through trying to resume the SJC.
// --------------------------------------------------------------------------

// resumer implements the Resumer interface.
type resumer struct {
	rm           Manager
	dbc          *sql.DB
	jrc          jr.Client
	defaultJRURL string
	host         string // the host this request manager is currently running on
	shutdownChan chan struct{}
	logger       *log.Entry
	sjcTTL       time.Duration // how long after being suspended do we keep an SJC
}

type ResumerConfig struct {
	RequestManager       Manager
	DBConnector          *sql.DB
	JRClient             jr.Client
	DefaultJRURL         string
	RMHost               string
	ShutdownChan         chan struct{}
	SuspendedJobChainTTL time.Duration
}

func NewResumer(cfg ResumerConfig) Resumer {
	return &resumer{
		rm:           cfg.RequestManager,
		dbc:          cfg.DBConnector,
		jrc:          cfg.JRClient,
		defaultJRURL: cfg.DefaultJRURL,
		host:         cfg.RMHost,
		shutdownChan: cfg.ShutdownChan,
		sjcTTL:       cfg.SuspendedJobChainTTL,
	}
}

// Suspend a running request and save its suspended job chain.
func (r *resumer) Suspend(sjc proto.SuspendedJobChain) (err error) {
	req, err := r.rm.Get(sjc.RequestId)
	if err != nil {
		return err
	}
	// We can only suspend a request that is currently running.
	if req.State != proto.STATE_RUNNING {
		return serr.NewErrInvalidState(proto.StateName[proto.STATE_RUNNING], proto.StateName[req.State])
	}

	rawSJC, err := json.Marshal(sjc)
	if err != nil {
		return fmt.Errorf("cannot marshal Suspended Job Chain: %s", err)
	}

	// Connect to database + start transaction.
	ctx := context.TODO()
	txn, err := r.dbc.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer txn.Rollback()

	// Insert the sjc into the suspended_job_chain table. The 'suspended_at' and
	// 'updated_at' columns will automatically be set to the current timestamp
	q := "INSERT INTO suspended_job_chains (request_id, suspended_job_chain) VALUES (?, ?)"
	_, err = txn.ExecContext(ctx, q,
		req.Id,
		rawSJC,
	)
	if err != nil {
		return err
	}

	// Mark request as suspended and set JR url to null. This will only update the
	// request if the current state is RUNNING (it should be, per the earlier test).
	req.State = proto.STATE_SUSPENDED
	req.JobRunnerURL = ""
	err = r.updateRequestWithTxn(req, proto.STATE_RUNNING, txn)
	if err != nil {
		// If we couldn't update the state's request to Suspended, we don't commit
		// the transaction that inserted the SJC into the db. We don't want to keep
		// an SJC for a request that wasn't marked as Suspended.
		return err
	}

	return txn.Commit()
}

// ResumeAll tries to resume all currently suspended job chains. All errors are
// logged, not returned - we want the resumer to keep running even if there's a
// one-time problem resuming requests.
func (r *resumer) ResumeAll() {
	ctx := context.TODO()

	// Retrieve IDs for all (unclaimed) SJCs.
	q := "SELECT request_id FROM suspended_job_chains WHERE rm_host IS NULL"
	rows, err := r.dbc.QueryContext(ctx, q)
	if err != nil {
		log.Errorf("error querying db for SJCs: %s", err)
		return
	}
	defer rows.Close()

	var sjcs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			log.Errorf("error scanning rows: %s", err)
			return
		}

		sjcs = append(sjcs, id)
	}

	// Shuffle the array of SJC IDs into random order.
	rand.Shuffle(len(sjcs), func(i, j int) {
		sjcs[i], sjcs[j] = sjcs[j], sjcs[i]
	})

	// Attempt to resume each SJC.
	for _, id := range sjcs {
		// If the resumer is shutting down, stop trying to resume requests.
		select {
		case <-r.shutdownChan:
			log.Infof("Request Manager is shutting down - not attempting to resume any more requests")
			return
		default:
		}

		// Try to claim the SJC, to indicate that this RM instance is attempting to
		// resume it. If we fail to claim the SJC, another RM must have already
		// claimed it - continue to the next SJC.
		claimed, err := r.claimSJC(id)
		if err != nil {
			log.Errorf("error claiming SJC %s: %s", id, err)
			continue
		}
		if !claimed {
			continue
		}

		err = r.Resume(id)
		if err != nil {
			log.Errorf("error resuming SJC %s: %s", id, err)
			// We didn't resume the SJC, so unclaim it.
			err := r.unclaimSJC(id, true)
			if err != nil {
				log.Errorf("error unclaiming SJC %s: %s", id, err)
				continue
			}
		}
	}
}

// Resume a request by sending it to the JR and updating its state.
func (r *resumer) Resume(id string) error {
	// Connect to database
	ctx := context.TODO()

	// Retrieve the request state.
	var state byte
	q := "SELECT state FROM requests WHERE request_id = ?"
	err := r.dbc.QueryRowContext(ctx, q, id).Scan(&state)
	if err != nil {
		return fmt.Errorf("error querying db for request state: %s", err)
	}

	// Only resume the SJC if this request is actually suspended.
	if state != proto.STATE_SUSPENDED {
		// Delete the SJC - we don't need to resume a request that isn't
		// suspended. Transactions make sure we can't catch Suspend()
		// in between saving an SJC and updating its request's state.
		// This state can happen if an SJC has already been resumed, but
		// the RM failed on deleting it from the db.
		log.Errorf("cannot resume SJC %s because request is not suspended (state = %s) - deleting SJC", id, proto.StateName[state])
		if err := r.deleteSJC(id); err != nil {
			return fmt.Errorf("error deleting SJC: %s", err)
		}
		return nil // no error - SJC was resumed earlier
	}

	// Retrieve the actual Suspended Job Chain
	var rawSJC []byte
	q = "SELECT suspended_job_chain FROM suspended_job_chains WHERE request_id = ? AND rm_host = ?"
	err = r.dbc.QueryRowContext(ctx, q, id, r.host).Scan(&rawSJC)
	if err != nil {
		return fmt.Errorf("error querying db for request state: %s", err)
	}

	var sjc proto.SuspendedJobChain
	err = json.Unmarshal(rawSJC, &sjc)
	if err != nil {
		return fmt.Errorf("error unmarshaling SJC: %s", err)
	}

	// Send suspended job chain to JR, which will resume running it.
	chainURL, err := r.jrc.ResumeJobChain(r.defaultJRURL, sjc)
	if err != nil {
		return fmt.Errorf("error sending SJC to Job Runner: %s", err)
	}

	// Update the request's state and save the JR url running it. Since we
	// previously checked that the request state was STATE_SUSPENDED, this
	// should always succeed.
	req := proto.Request{
		Id:           id,
		State:        proto.STATE_RUNNING,
		JobRunnerURL: strings.TrimSuffix(chainURL.String(), chainURL.RequestURI()),
	}
	if err = r.updateRequest(req, proto.STATE_SUSPENDED); err != nil {
		return fmt.Errorf("error setting request state to STATE_RUNNING and saving job runner url: %s", err)
	}

	// Now that we've resumed running the request, we can delete the SJC. We don't
	// do this within the same transaction as updating the request, because even if
	// deleting the SJC fails, the job chain has already been sent to the JR and we
	// want to update the request state to Running.
	if err := r.deleteSJC(id); err != nil {
		return fmt.Errorf("error deleting SJC: %s", err)
	}

	return nil
}

// Two parts: cleaning up abanoned SJCs and cleaning up old SJCs
// Abandoned SJCs have been claimed by an RM but have not been updated in a while
// (the RM probably crashed) - unclaim them so they can be resumed in the future.
// It's possible one of these SJCs was already resumed (just not deleted), but
// ResumeAll will take care of deleting it the next time it runs.
// Old SJCs were suspended a long time ago but never resumed, and they've exceeded
// the SJC TTL. Mark their requests as FAILED (if they're currently SUSPENDED) and
// remove the SJCs from the db.
func (r *resumer) Cleanup() {
	// Connect to database
	ctx := context.TODO()

	// Clean up abandoned SJCS:

	// Retrieve request ID for all claimed SJCs that haven't been updated in the
	// last 5 minutes. The RM that claimed them probably encountered some problem
	// (eg. it crashed), so we want to make sure they aren't claimed forever.
	q := "SELECT request_id FROM suspended_job_chains WHERE rm_host IS NOT NULL AND updated_at < NOW() - INTERVAL 5 MINUTE"
	rows, err := r.dbc.QueryContext(ctx, q)
	if err != nil {
		log.Errorf("error querying db: %s", err)
		return
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var reqId string
		if err := rows.Scan(&reqId); err != nil {
			log.Errorf("error scanning row: %s", err)
			return
		}

		ids = append(ids, reqId)
	}
	rows.Close() // must close before new queries to unclaim SJCs

	for _, reqId := range ids {
		reqLogger := log.WithFields(log.Fields{"request": reqId})

		// Unclaim SJC so another RM can resume it.
		err = r.unclaimSJC(reqId, false)
		if err != nil {
			reqLogger.Errorf("error unclaiming SJC: %s", err)
		}
	}

	// Clean up old SJCs:

	// Retrieve Request IDs of all unclaimed SJCs suspended more than 1 hour ago.
	ttlSeconds := fmt.Sprintf("%.0f", r.sjcTTL.Round(time.Second).Seconds())
	q = "SELECT request_id FROM suspended_job_chains WHERE rm_host IS NULL AND suspended_at < NOW() - INTERVAL ? SECOND"
	rows, err = r.dbc.QueryContext(ctx, q, ttlSeconds)
	if err != nil {
		log.Errorf("error querying db: %s", err)
		return
	}
	defer rows.Close()

	var requests []proto.Request
	for rows.Next() {
		var req proto.Request
		if err := rows.Scan(&req.Id); err != nil {
			log.Errorf("error scanning row: %s", err)
			return
		}

		requests = append(requests, req)
	}
	rows.Close() // must be closed before making new queries

	// Delete the SJCs for all of these requests. If the request state is Suspended,
	// mark it as Failed.
	for _, req := range requests {
		reqLogger := log.WithFields(log.Fields{"request": req.Id})

		// Claim the SJC so an RM doesn't try to resume it while we're deleting it.
		claimed, err := r.claimSJC(req.Id)
		if err != nil {
			reqLogger.Errorf("error claiming SJC: %s", err)
			continue
		}
		if !claimed {
			// Couldn't claim SJC - an RM is trying to resume it.
			continue
		}

		// Change the request state to Failed if it's currently Suspended. The
		// request might not be Suspended if an RM did resume the request, but
		// failed on deleting the SJC. Update will return ErrNotUpdated in this
		// case - ignore this error.
		req.State = proto.STATE_FAIL
		req.JobRunnerURL = ""
		err = r.updateRequest(req, proto.STATE_SUSPENDED)
		if err != nil && err != ErrNotUpdated {
			reqLogger.Errorf("error changing request state from SUSPENDED to FAILED: %s", err)
			if err := r.unclaimSJC(req.Id, true); err != nil {
				reqLogger.Errorf("error unclaiming SJC: %s", err)
			}
			continue
		}

		// Delete the old SJC. If this fails, the SJC will get deleted the next time
		// an RM tries to resume it (since the request is now marked Failed).
		err = r.deleteSJC(req.Id)
		if err != nil {
			reqLogger.Errorf("error deleting SJC: %s", err)
			if err := r.unclaimSJC(req.Id, true); err != nil {
				reqLogger.Errorf("error unclaiming SJC: %s", err)
			}
			continue
		}
	}

	return
}

// Update the State and JR url of a request. This is a wrapper around
// updateRequestWithTxn that creates a transaction for updating the request.
func (r *resumer) updateRequest(request proto.Request, curState byte) error {
	// Start a transaction.
	txn, err := r.dbc.BeginTx(context.TODO(), nil)
	if err != nil {
		return err
	}
	defer txn.Rollback()

	err = r.updateRequestWithTxn(request, curState, txn)
	if err != nil {
		return err
	}

	return txn.Commit()
}

// updateRequestWithTxn updates the State and JR url as set in the given request,
// using the provided db transaction. The request is updated only if its current
// state in the db matches the state provided.
func (r *resumer) updateRequestWithTxn(request proto.Request, curState byte, txn *sql.Tx) error {
	// If JobRunnerURL is empty, we want to set the db field to NULL (not an empty string).
	var jrURL interface{}
	if request.JobRunnerURL != "" {
		jrURL = request.JobRunnerURL
	}

	// Update the 'state' and 'jr_url' fields only.
	q := "UPDATE requests SET state = ?, jr_url = ? WHERE request_id = ? AND state = ?"
	res, err := txn.Exec(q, request.State, jrURL, request.Id, curState)
	if err != nil {
		return err
	}

	cnt, err := res.RowsAffected()
	if err != nil {
		return err
	}

	switch cnt {
	case 0:
		// Either the request's current state != curState, or no request with the
		// id given exists.
		return ErrNotUpdated
	case 1:
		return nil
	default:
		// This should be impossible since we specify the primary key (request id)
		// in the WHERE clause of the update.
		return ErrMultipleUpdated
	}
}

// deleteSJC removes an SJC from the db. The RM needs to have claimed the
// SJC in order to delete it.
func (r *resumer) deleteSJC(requestId string) error {
	q := "DELETE FROM suspended_job_chains WHERE request_id = ? AND rm_host = ?"
	result, err := r.dbc.Exec(q, requestId, r.host)
	if err != nil {
		return err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return err
	}

	switch count {
	case 0:
		return fmt.Errorf("cannot find SJC to delete - may not be claimed by RM")
	case 1: // Success
	default:
		// This should be impossible since we specify the primary key (request id)
		// in the WHERE clause of the update.
		return ErrMultipleUpdated
	}
	return nil
}

// Claim that this RM will resume the SJC corresponding to the request id.
// Returns true if SJC is successfully claimed (this RM should resume the SJC);
// returns false if failed to claim SJC (another RM is resuming the SJC).
// claimSJC is used to make sure that only one RM is trying to resume a specific
// SJC at any given time.
func (r *resumer) claimSJC(requestId string) (bool, error) {
	// Claim SJC by setting rm_host to the host of this RM. Only claim if not
	// alread claimed (rm_host = NULL).
	q := "UPDATE suspended_job_chains SET rm_host = ? WHERE request_id = ? AND rm_host IS NULL"
	result, err := r.dbc.ExecContext(context.TODO(), q, r.host, requestId)
	if err != nil {
		return false, err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	switch count {
	case 0:
		// Failed to claim SJC: another RM had already claimed the SJC (rm_host was
		// not NULL) or the SJC does not exist in the table (already resumed).
		return false, nil
	case 1: // Success
		return true, nil
	default:
		// This should be impossible since we specify the primary key (request id)
		// in the WHERE clause of the update.
		return true, ErrMultipleUpdated
	}
}

// Un-claim the SJC corresponding to the request id - indicates an RM is no longer
// trying to resume this SJC - so it may be claimed again later.
// strict = true requires that this RM has a claim on the SJC, or unclaimSJC
// will return an error. Strict should always be used except when cleaning up
// abandoned SJCs.
func (r *resumer) unclaimSJC(requestId string, strict bool) error {
	// Unclaim SJC by setting rm_host back to NULL.
	var result sql.Result
	var err error
	if strict { // require that this RM has the SJC claimed
		q := "UPDATE suspended_job_chains SET rm_host = NULL WHERE request_id = ? AND rm_host = ?"
		result, err = r.dbc.ExecContext(context.TODO(), q, requestId, r.host)
	} else {
		q := "UPDATE suspended_job_chains SET rm_host = NULL WHERE request_id = ?"
		result, err = r.dbc.ExecContext(context.TODO(), q, requestId)
	}
	if err != nil {
		return err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return err
	}

	switch count {
	case 0:
		return errors.New("could not find SJC to unclaim - either no SJC exists for this request, or this RM instance has not claimed the SJC")
	case 1: // Success
		return nil
	default:
		// This should be impossible since we specify the primary key (request id)
		// in the WHERE clause of the update.
		return ErrMultipleUpdated
	}
}
