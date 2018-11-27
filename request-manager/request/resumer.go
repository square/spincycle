// Copyright 2018, Square, Inc.

package request

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	myconn "github.com/go-mysql/conn"

	jr "github.com/square/spincycle/job-runner"
	"github.com/square/spincycle/proto"
	"github.com/square/spincycle/request-manager/db"
)

type Resumer interface {
	// Run periodically attempts to resume any Suspended Job Chains stored in
	// the database, and also cleans up any SJCs that get into a bad state. Run
	// blocks until the Resumer is shut down by closing shutdownChan.
	Run()

	// Suspend marks a running request as suspended and saves the corresponding
	// suspended job chain.
	Suspend(requestId string, sjc proto.SuspendedJobChain) error
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

const (
	// How often we try to resume any suspended job chains.
	ResumeInterval = 10 * time.Second
)

// resumer implements the Resumer interface.
type resumer struct {
	rm             Manager
	dbc            myconn.Connector
	jrc            jr.Client
	host           string // the host this request manager is currently running on
	shutdownChan   chan struct{}
	resumeInterval time.Duration
	logger         *log.Entry
}

type ResumerConfig struct {
	RequestManager Manager
	DBConnector    myconn.Connector
	JRClient       jr.Client
	RMHost         string
	ShutdownChan   chan struct{}
	ResumeInterval time.Duration
}

func NewResumer(cfg ResumerConfig) Resumer {
	return &resumer{
		rm:             cfg.RequestManager,
		dbc:            cfg.DBConnector,
		jrc:            cfg.JRClient,
		host:           cfg.RMHost,
		shutdownChan:   cfg.ShutdownChan,
		resumeInterval: cfg.ResumeInterval,
	}
}

// Run tries to resume and cleanup all SJCs every 10 seconds until shutdownChan
// is closed.
func (r *resumer) Run() {
	for {
		select {
		case <-r.shutdownChan:
			return
		case <-time.After(r.resumeInterval):
			r.resumeAll()

			// Unclaim SJCs that another RM has had claimed for a long time.
			r.cleanupAbandonedSJCs()

			// Delete SJCs that haven't been resumed within an hour.
			r.cleanupOldSJCs()
		}
	}
}

// Suspend a running request and save its suspended job chain.
func (r *resumer) Suspend(requestId string, sjc proto.SuspendedJobChain) (err error) {
	sjc.RequestId = requestId // make sure the SJC's request id = request id given

	req, err := r.rm.Get(requestId)
	if err != nil {
		return err
	}
	// We can only suspend a request that is currently running.
	if req.State != proto.STATE_RUNNING {
		return NewErrInvalidState(proto.StateName[proto.STATE_RUNNING], proto.StateName[req.State])
	}

	rawSJC, err := json.Marshal(sjc)
	if err != nil {
		return fmt.Errorf("cannot marshal Suspended Job Chain: %s", err)
	}

	// Connect to database + start transaction.
	ctx := context.TODO()
	conn, err := r.dbc.Open(ctx)
	if err != nil {
		return err
	}
	defer r.dbc.Close(conn)

	txn, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer txn.Rollback()

	// Insert the sjc into the suspended_job_chain table. The 'suspended_at' and
	// 'last_updated_at' columns will automatically be set to the current timestamp
	q := "INSERT INTO suspended_job_chains (request_id, suspended_job_chain) VALUES (?, ?)"
	_, err = txn.ExecContext(ctx, q,
		requestId,
		rawSJC,
	)
	if err != nil {
		return err
	}

	// Mark request as suspended and set JR host to null. This will only update the
	// request if the current state is RUNNING (it should be, per the earlier test).
	req.State = proto.STATE_SUSPENDED
	req.JobRunnerHost = nil
	err = r.updateRequestWithTxn(req, proto.STATE_RUNNING, txn)
	if err != nil {
		// If we couldn't update the state's request to Suspended, we don't commit
		// the transaction that inserted the SJC into the db. We don't want to keep
		// an SJC for a request that wasn't marked as Suspended.
		return err
	}

	return txn.Commit()
}

// resumeAll tries to resume all currently suspended job chains. All errors are
// logged, not returned - we want the resumer to keep running even if there's a
// one-time problem resuming requests.
func (r *resumer) resumeAll() {
	// Connect to database
	ctx := context.TODO()
	conn, err := r.dbc.Open(ctx)
	if err != nil {
		log.Errorf("could not connect to database: %s", err)
		return
	}
	defer r.dbc.Close(conn)

	// Retrieve Request IDs + states for all (unclaimed) SJCs, in random order.
	q := "SELECT requests.request_id, requests.state, sjcs.suspended_job_chain FROM suspended_job_chains sjcs JOIN requests ON sjcs.request_id = requests.request_id WHERE sjcs.rm_host IS NULL ORDER BY RAND()"
	rows, err := conn.QueryContext(ctx, q)
	if err != nil {
		log.Errorf("error querying db for SJCs: %s", err)
		return
	}
	defer rows.Close()

	var requests []proto.Request
	sjcs := make(map[string]proto.SuspendedJobChain) // request id -> SJC
	for rows.Next() {
		var req proto.Request
		var rawSJC []byte
		if err := rows.Scan(&req.Id, &req.State, &rawSJC); err != nil {
			log.Errorf("error scanning rows: %s", err)
			return
		}

		var sjc proto.SuspendedJobChain
		if err := json.Unmarshal(rawSJC, &sjc); err != nil {
			log.Errorf("cannot unmarshal suspended job chain (req id = %s): %s", req.Id, err)
			return
		}

		requests = append(requests, req)
		sjcs[req.Id] = sjc
	}

	// Attempt to resume each SJC.
	for _, req := range requests {
		// If the resumer is shutting down, stop trying to resume requests.
		select {
		case <-r.shutdownChan:
			log.Infof("Request Manager is shutting down - not attempting to resume any more requests")
			return
		default:
		}

		r.resume(req, sjcs[req.Id], conn)
	}
}

// Attempts to resume a request. Log all errors instead of returning them. We
// don't care if we fail to resume the request - it'll be tried again later.
func (r *resumer) resume(req proto.Request, sjc proto.SuspendedJobChain, conn *sql.Conn) {
	reqLogger := log.WithFields(log.Fields{"request": req.Id})

	// Try to claim the SJC, to indicate that this RM instance is attempting to
	// resume it. If we fail to claim the SJC, another RM must have already
	// claimed it - continue to the next request.
	claimed, err := r.claimSJC(req.Id, conn)
	if err != nil {
		reqLogger.Errorf("error claiming SJC: %s", err)
		return
	}
	if !claimed {
		return
	}

	// Always unclaim the SJC, so it can be tried again later.
	defer func() {
		if claimed {
			err := r.unclaimSJC(req.Id, true, conn)
			if err != nil {
				reqLogger.Errorf("error unclaiming SJC: %s", err)
			}
		}
	}()

	// Only resume the SJC if this request is actually suspended.
	if req.State != proto.STATE_SUSPENDED {
		// Delete the SJC - we don't need to resume a request that isn't
		// suspended. Transactions make sure we can't catch Suspend()
		// in between saving an SJC and updating its request's state.
		reqLogger.Errorf("cannot resume request because request is not suspended (state = %s) - deleting SJC", proto.StateName[req.State])
		if err := r.deleteSJC(req.Id, conn); err != nil {
			reqLogger.Errorf("error deleting SJC: %s", err)
			return
		}
		claimed = false // deleted the SJC, so don't need to unclaim it
		return
	}

	// Send suspended job chain to JR, which will resume running it.
	host, err := r.jrc.ResumeJobChain(sjc)
	if err != nil {
		reqLogger.Warnf("could not send SJC to Job Runner: %s", err)
		return
	}

	// Update the request's state and save the JR host running it. Since we
	// previously checked that the request state was STATE_SUSPENDED, this
	// should always succeed.
	req.State = proto.STATE_RUNNING
	req.JobRunnerHost = &host
	if err = r.updateRequest(req, proto.STATE_SUSPENDED, conn); err != nil {
		reqLogger.Errorf("error setting request state to STATE_RUNNING and saving job runner hostname: %s", err)
		return
	}

	// Now that we've resumed running the request, we can delete the SJC. We don't
	// do this within the same transaction as updating the request, because even if
	// deleting the SJC fails, the job chain has already been sent to the JR and we
	// want to update the request state to Running.
	if err := r.deleteSJC(req.Id, conn); err != nil {
		reqLogger.Errorf("error deleting SJC: %s", err)
		return
	}
	claimed = false // we've deleted the SJC, so no longer need to unclaim it
}

// Find SJCs that have been claimed by an RM but have not been updated in a while
// (the RM probably crashed) and unclaim them so they can be resumed in the future.
// It's possible one of these SJCs was already resumed, but resumeAll will take
// care of deleting it next time it runs.
func (r *resumer) cleanupAbandonedSJCs() {
	// Connect to database
	ctx := context.TODO()
	conn, err := r.dbc.Open(ctx)
	if err != nil {
		log.Errorf("could not connect to database: %s", err)
		return
	}
	defer r.dbc.Close(conn)

	// Retrieve request ID for all claimed SJCs that haven't been updated in the
	// last 5 minutes. The RM that claimed them probably encountered some problem
	// (eg. it crashed), so we want to make sure they aren't claimed forever.
	q := "SELECT request_id FROM suspended_job_chains WHERE rm_host IS NOT NULL AND last_updated_at < NOW() - INTERVAL 5 MINUTE"
	rows, err := conn.QueryContext(ctx, q)
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
		err = r.unclaimSJC(reqId, false, conn)
		if err != nil {
			reqLogger.Errorf("error unclaiming SJC: %s", err)
		}
	}

	return
}

// Clean up SJCs that were suspended a long time ago but were never resumed.
// Mark their requests as FAILED (if they're currently SUSPENDED) and remove
// the SJCs from the db. This also takes care of SJCs that were resumed, but
// didn't get deleted correctly.
func (r *resumer) cleanupOldSJCs() {
	// Connect to database
	ctx := context.TODO()
	conn, err := r.dbc.Open(ctx)
	if err != nil {
		log.Errorf("could not connect to database: %s", err)
		return
	}
	defer r.dbc.Close(conn)

	// Retrieve Request IDs of all unclaimed SJCs suspended more than 1 hour ago.
	q := "SELECT request_id FROM suspended_job_chains WHERE rm_host IS NULL AND suspended_at < NOW() - INTERVAL 1 HOUR"
	rows, err := conn.QueryContext(ctx, q)
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

		claimed, err := r.claimSJC(req.Id, conn)
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
		// failed on deleting the SJC. Update will return db.ErrNotUpdated in this
		// case - ignore this error.
		req.State = proto.STATE_FAIL
		req.JobRunnerHost = nil
		err = r.updateRequest(req, proto.STATE_SUSPENDED, conn)
		if err != nil && err != db.ErrNotUpdated {
			reqLogger.Errorf("error changing request state from SUSPENDED to FAILED: %s", err)
			if err := r.unclaimSJC(req.Id, true, conn); err != nil {
				reqLogger.Errorf("error unclaiming SJC: %s", err)
			}
			continue

		}

		// Delete the old SJC. If this fails, the SJC will get deleted the next time
		// an RM tries to resume it (since the request is now marked Failed).
		err = r.deleteSJC(req.Id, conn)
		if err != nil {
			reqLogger.Errorf("error deleting SJC: %s", err)
			if err := r.unclaimSJC(req.Id, true, conn); err != nil {
				reqLogger.Errorf("error unclaiming SJC: %s", err)
			}
			continue
		}
	}

	return
}

// Update the State and JR host of a request. This is a wrapper around
// updateRequestWithTxn that creates a transaction for updating the request.
func (r *resumer) updateRequest(request proto.Request, curState byte, conn *sql.Conn) error {
	// Start a transaction.
	txn, err := conn.BeginTx(context.TODO(), nil)
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

// updateRequestWithTxn updates the State and JR host as set in the given request,
// using the provided db transaction. The request is updated only if its current
// state in the db matches the state provided.
func (r *resumer) updateRequestWithTxn(request proto.Request, curState byte, txn *sql.Tx) error {
	// Update the 'state' and 'jr_host' fields only.
	q := "UPDATE requests SET state = ?, jr_host = ? WHERE request_id = ? AND state = ?"
	res, err := txn.Exec(q, request.State, request.JobRunnerHost, request.Id, curState)
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
		return db.ErrNotUpdated
	case 1:
		return nil
	default:
		// This should be impossible since we specify the primary key (request id)
		// in the WHERE clause of the update.
		return db.ErrMultipleUpdated
	}
}

// deleteSJC removes an SJC from the db. The RM needs to have claimed the
// SJC in order to delete it.
func (r *resumer) deleteSJC(requestId string, conn *sql.Conn) error {
	// Start a transaction.
	txn, err := conn.BeginTx(context.TODO(), nil)
	if err != nil {
		return err
	}
	defer txn.Rollback()

	q := "DELETE FROM suspended_job_chains WHERE request_id = ? AND rm_host = ?"
	result, err := txn.Exec(q,
		requestId,
		r.host)
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
		log.Infof("deleted sjc %s", requestId)
		return txn.Commit()
	default:
		// This should be impossible since we specify the primary key (request id)
		// in the WHERE clause of the update.
		return db.ErrMultipleUpdated
	}
}

// Claim that this RM will resume the SJC corresponding to the request id.
// Returns true if SJC is successfully claimed (this RM should resume the SJC);
// returns false if failed to claim SJC (another RM is resuming the SJC).
// claimSJC is used to make sure that only one RM is trying to resume a specific
// SJC at any given time.
func (r *resumer) claimSJC(requestId string, conn *sql.Conn) (bool, error) {
	// Start a transaction
	ctx := context.TODO()
	txn, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer txn.Rollback()

	// Claim SJC by setting rm_host to the host of this RM. Only claim if not
	// alread claimed (rm_host = NULL).
	q := "UPDATE suspended_job_chains SET rm_host = ? WHERE request_id = ? AND rm_host IS NULL"
	result, err := txn.ExecContext(ctx, q,
		r.host,
		requestId,
	)
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
		return false, txn.Commit()
	case 1: // Success
		return true, txn.Commit()
	default:
		// This should be impossible since we specify the primary key (request id)
		// in the WHERE clause of the update.
		return true, db.ErrMultipleUpdated
	}
}

// Un-claim the SJC corresponding to the request id - indicates an RM is no longer
// trying to resume this SJC - so it may be claimed again later.
// strict = true requires that this RM has a claim on the SJC, or unclaimSJC
// will return an error. Strict should always be used except when cleaning up
// abandoned SJCs.
func (r *resumer) unclaimSJC(requestId string, strict bool, conn *sql.Conn) error {
	// Start a transaction
	ctx := context.TODO()
	txn, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer txn.Rollback()

	// Unclaim SJC by setting rm_host back to NULL.
	var result sql.Result
	if strict { // require that this RM has the SJC claimed
		q := "UPDATE suspended_job_chains SET rm_host = NULL WHERE request_id = ? AND rm_host = ?"
		result, err = txn.ExecContext(ctx, q,
			requestId,
			r.host,
		)
	} else {
		q := "UPDATE suspended_job_chains SET rm_host = NULL WHERE request_id = ?"
		result, err = txn.ExecContext(ctx, q,
			requestId,
		)
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
		return txn.Commit()
	default:
		// This should be impossible since we specify the primary key (request id)
		// in the WHERE clause of the update.
		return db.ErrMultipleUpdated
	}
}
