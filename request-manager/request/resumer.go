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
	// Run starts running the Resumer's resume loop, which periodically calls
	// ResumeAll to resume any Suspended Job Chains stored in its database. Run
	// blocks until the resumer is shut down (by closing shutdownChan).
	Run()

	// ResumeAll tries to resumes all Suspended Job Chains and cleans up any SJCs
	// or requests that have been suspended for too long. All errors are logged,
	// not returned.
	ResumeAll()

	// Cleanup cleans up any old or abandoned SJCs and any suspended requests that
	// don't have an SJC. Old SJCs (suspended more than 1 hour ago) are deleted,
	// abandoned SJCs (claimed by an RM but not resumed) are unclaimed so they can
	// be resumed, and suspended requests without SJCs are marked as failed.
	Cleanup()

	// Suspend marks a running request as suspended and saves the corresponding
	// suspended job chain.
	Suspend(requestId string, sjc proto.SuspendedJobChain) error
}

// --------------------------------------------------------------------------
// Lifecycle of a Suspended Job Chain:
//
//  When a Job Runner wants to suspend a job chain it's running, it sends a
//  Suspended Job Chain, containing all the info necessary to recreate the
//  job chain it's running, to the Request Manager. The request Resumer then
//  saves this SJC and marks the corresponding request as Suspended.
//
//  Every 5 seconds, the request Resumer loops over all of the Suspended Job
//  Chains it has saved, in a random order. It tries to resume each one by:
//   - claiming the SJC, so no other Resumer can try to resume it
//   - sending the SJC to a Job Runner
//   - updating the state of the request as RUNNING (+ saving which JR host it's
//     running on)
//   - deleting the SJC
//  If the Resumer fails at any step, it unclaims the SJC before continuing, so
//  another Resumer can try again to resume this request later.
//
//  Once the Resumer has looped over all of the Suspended Job Chains, it performs
//  some cleanup. Any SJCs that have been claimed by other Resumers but haven't
//  been updated in a while are unclaimed - the Resumer using these probably
//  encountered a problem. Any SJCs more than an hour old are deleted, and their
//  requests marked as failed. Any requests that are marked as SUSPENDED but don't
//  have a corresponding SJC are marked as failed.
// --------------------------------------------------------------------------

// (my notes)
//
// Suspend:
// 1: Resumer receives a request ID + SJC.
// 2: Resumer checks that request's state = RUNNING. If not, it returns an error
// 3: Resumer saves the SJC in its database.
// 4: Resumer sets the request's state to SUSPENDED. If not able to, deletes the
//    SJC it just saved, and returns an error.
//
// Resume:
// Every 5 seconds:
//  Resumer retrieves the request ID of all SJCs, in a random order.
//  For each request ID:
//   - check that request state = SUSPENDED. If not, continue. We don't
//     delete the SJC, since it's possible that the request was just suspended,
//     and the SJC has been saved but the request state hasn't been updated.
//   - claim the SJC by setting rm_host = this RM host. If we fail to claim the
//     SJC (meaning it was claimed by another Resumer), continue.
//   - send the SJC to a Job Runner to be resumed. If we get an error, retry a few
//     times. If we still end up with an error, unclaim the SJC and continue.
//   - update the state of the request to RUNNING (+ set jr_host to the JR running
//     it). If this fails, it means the state of the request was not SUSPENDED. We
//     checked to make sure the state was SUSPENDED earlier, so this shouldn't
//     happen. If it does happen, log an error and keep going, since we still want
//     to delete the SJC.
//   - delete the SJC. We shouldn't get an error doing this, but if we do, log it
//     and continue.
//  Once we're done looping through all the SJCs, do some cleanup:
//   - Get all SJCs that are claimed but haven't been updated in the last minute.
//     For each: if it's request is SUSPENDED, unclaim it. Otherwise delete it.
//   - Get all SJCs that are unclaimed but were suspended more than an hour ago.
//     Delete all of them and, if their requests are SUSPENDED, mark them FAILED.
//   - Get all requests that are SUSPENDED but don't have an SJC saved. Mark them
//     as FAILED.
//
//

const (
	// How often we try to resume any suspended job chains.
	ResumeInterval = 10 * time.Second
)

var (
	ErrShuttingDown = errors.New("Request Manager is shutting down - cannot suspend requests")

	ErrNotUnclaimed = errors.New("could not find SJC to unclaim - either no SJC exists for this request, or this RM instance has not claimed the SJC")
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

// Run starts running the resumer loop, which periodically attempts to
// resume any SJCs in the db, until the resumer is shut down by closing
// shutdownChan.
func (r *resumer) Run() {
	for {
		select {
		case <-r.shutdownChan:
			return
		case <-time.After(r.resumeInterval):
			r.ResumeAll()
			r.Cleanup()
		}
	}
}

// Suspend a running request and save its suspended job chain.
func (r *resumer) Suspend(requestId string, sjc proto.SuspendedJobChain) (err error) {
	logger := log.WithFields(log.Fields{"requestId": requestId})
	logger.Infof("suspending request")

	// Log whether we succeeded before returning.
	defer func() {
		if err != nil { // named return value
			logger.Errorf("failed to suspend request: %s", err)
		} else {
			logger.Infof("successfully suspended request")
		}
	}()

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
	// request if the current state is RUNNING. If this fails, the request's state
	// has been changed since we checked it.
	req.State = proto.STATE_SUSPENDED
	req.JobRunnerHost = nil
	err = r.updateRequestWithTxn(req, proto.STATE_RUNNING, txn)
	if err != nil {
		// If we couldn't update the state's request to Suspended, we don't commit
		// the transaction that inserted the SJC into the db. We don't want to keep
		// an SJC for a request that wasn't marked as Suspended.
		return err
	}

	err = txn.Commit()
	if err != nil {
		return err
	}

	return nil
}

// Attempt to resume all currently suspended job chains (and corresponding
// requests).
func (r *resumer) ResumeAll() {
	log.Infof("attempting to resume suspended requests")
	defer log.Infof("done resuming suspended requests")

	// Connect to database
	ctx := context.TODO()
	conn, err := r.dbc.Open(ctx)
	if err != nil {
		log.Errorf("could not connect to database: %s", err)
		return
	}
	defer r.dbc.Close(conn) // don't leak conn

	// Retrieve Request IDs + states for all (unclaimed) SJCs, in random order.
	q := "SELECT requests.request_id, requests.state FROM suspended_job_chains sjcs JOIN requests ON sjcs.request_id = requests.request_id WHERE sjcs.rm_host IS NULL ORDER BY RAND()"
	rows, err := conn.QueryContext(ctx, q)
	if err != nil {
		log.Errorf("error querying db for SJCs: %s", err)
		return
	}
	defer rows.Close()

	var requests []proto.Request
	for rows.Next() {
		var req proto.Request
		if err := rows.Scan(&req.Id, &req.State); err != nil {
			log.Errorf("error scanning rows: %s", err)
			return
		}

		requests = append(requests, req)
	}
	rows.Close() // must close before making more queries on this connection

	// Attempt to resume each SJC.
	for _, req := range requests {
		// If the resumer is shutting down, stop trying to resume requests.
		select {
		case <-r.shutdownChan:
			log.Infof("Request Manager is shutting down - not attempting to resume any more requests")
			return
		default:
		}

		// Only resume the SJC if its request is actually suspended.
		if req.State != proto.STATE_SUSPENDED {
			// Delete the SJC - we don't need to resume a request that isn't
			// suspended. MySQL transactions make sure we aren't catching Suspend()
			// in between saving the SJC and updating the request state.
			reqLogger := log.WithFields(log.Fields{"request": req.Id})
			reqLogger.Errorf("cannot resume request because request is not suspended (state = %s) - deleting SJC", proto.StateName[req.State])
			if err := r.deleteSJC(req.Id, conn); err != nil {
				reqLogger.Errorf("error deleting SJC: %s", err)
			}
			continue
		}

		r.resume(req.Id, conn)
	}
}

// Cleanup cleans up old SJCs and SJCs that have been abandoned by another RM. SJCs
// more than 1 hour old are deleted and their request marked as Failed. SJCs that
// have been claimed by an RM but never resumed are unclaimed so that another RM
// may try to resume them.
func (r *resumer) Cleanup() {
	// Unclaim or delete SJCs that another RM has had claimed for a long time.
	r.cleanupAbandonedSJCs()

	// Delete SJCs that haven't been resumed within an hour.
	r.cleanupOldSJCs()
}

// Attemps to resume a request. The resumer doesn't care whether the request
// actually gets resumed (it'll be tried again later), so we log all errors instead
// of returning them.
func (r *resumer) resume(requestId string, conn *sql.Conn) {
	reqLogger := log.WithFields(log.Fields{"request": requestId})

	// Always unclaim a claimed SJC, so it can be tried again later.
	var claimed bool
	defer func() {
		if claimed {
			err := r.unclaimSJC(requestId, true, conn)
			if err != nil {
				reqLogger.Errorf("error unclaiming SJC: %s", err)
			}
			reqLogger.Infof("unclaimed SJC")
		}
	}()

	// Try to claim the SJC, to indicate that this RM instance is attempting to
	// resume it. If we fail to claim the SJC, another RM must have already
	// claimed it - continue to the next request.
	var err error
	claimed, err = r.claimSJC(requestId, conn)
	if err != nil {
		reqLogger.Errorf("error claiming SJC: %s", err)
		return
	}
	if !claimed {
		return
	}
	reqLogger.Infof("claimed SJC")

	sjc, err := r.getClaimedSJC(requestId, conn)
	if err != nil {
		reqLogger.Errorf("error retrieving claimed SJC: %s", err)
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
	req := proto.Request{
		Id:            requestId,
		State:         proto.STATE_RUNNING,
		JobRunnerHost: &host,
	}
	if err = r.updateRequest(req, proto.STATE_SUSPENDED, conn); err != nil {
		reqLogger.Errorf("error setting request state to STATE_RUNNING and saving job runner hostname: %s", err)
		return
	}
	reqLogger.Infof("resumed request")

	// Now that we've resumed running the request, we can delete the SJC. We don't
	// do this within the same transaction as updating the request, because even if
	// deleting the SJC fails, the job chain has already been sent to the JR and we
	// want to update the request state to Running.
	if err := r.deleteSJC(requestId, conn); err != nil {
		reqLogger.Errorf("error deleting SJC: %s", err)
		return
	}
	claimed = false // we've deleted the SJC, so no longer need to unclaim it
	reqLogger.Infof("deleted SJC")
}

// Find SJCs that have been claimed by an RM but have not been updated in a while.
// Determine whether the request was already resumed and remove the SJC from db if
// so, or unclaim it to be resumed in the future.
func (r *resumer) cleanupAbandonedSJCs() {
	log.Infof("cleaning up abandoned SJCs")

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
	// eg. it crashed), so we want to make sure they aren't claimed forever.
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
	log.Infof("cleaning up old SJCs")

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
		// case - check for this and ignore it.
		req.State = proto.STATE_FAIL
		req.JobRunnerHost = nil
		err = r.updateRequest(req, proto.STATE_SUSPENDED, conn)
		if err != nil && err != db.ErrNotUpdated {
			reqLogger.Errorf("error changing request state from SUSPENDED to FAILED: %s", err)
			continue

		}

		// Delete the old SJC. If this fails, the SJC will get deleted the next time
		// an RM tries to resume it (since the request is now marked Failed).
		err = r.deleteSJC(req.Id, conn)
		if err != nil {
			reqLogger.Errorf("error deleting SJC: %s", err)
			continue
		}
	}

	return
}

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
		// Either the request;s current state != curState, or no request with the
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

// Remove an SJC that from the db. The RM does not need to have claimed the SJC to
// delete it.
func (r *resumer) deleteSJC(requestId string, conn *sql.Conn) error {
	// Start a transaction.
	txn, err := conn.BeginTx(context.TODO(), nil)
	if err != nil {
		return err
	}
	defer txn.Rollback()

	q := "DELETE FROM suspended_job_chains WHERE request_id = ?"
	result, err := txn.Exec(q, requestId)
	if err != nil {
		return err
	}

	count, err := result.RowsAffected()
	if err != nil {
		return err
	}

	switch count {
	case 0:
		return fmt.Errorf("cannot find SJC to delete")
	case 1: // Success
		return txn.Commit()
	default:
		// This should be impossible since we specify the primary key (request id)
		// in the WHERE clause of the update.
		return db.ErrMultipleUpdated
	}
}

// Retrieve an SJC that this RM has claimed using the given request id.
func (r *resumer) getClaimedSJC(requestId string, conn *sql.Conn) (proto.SuspendedJobChain, error) {
	var sjc proto.SuspendedJobChain

	var rawSJC []byte
	q := "SELECT suspended_job_chain FROM suspended_job_chains WHERE request_id = ? AND rm_host = ?"
	if err := conn.QueryRowContext(context.TODO(), q,
		requestId,
		r.host,
	).Scan(&rawSJC); err != nil {
		switch err {
		case sql.ErrNoRows:
			// The SJC doesn't exist or the RM hasn't claimed it.
			return sjc, fmt.Errorf("cannot retrieve suspended job chain (req id = %s) - does not exist, or RM has not claimed it", requestId)
		default:
			return sjc, err
		}
	}

	if err := json.Unmarshal(rawSJC, &sjc); err != nil {
		return sjc, fmt.Errorf("cannot unmarshal suspended job chain (req id = %s): %s", requestId, err)
	}

	return sjc, nil
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
// strict = true requires that this RM has a claim on the SJC.
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
		return ErrNotUnclaimed
	case 1: // Success
		return txn.Commit()
	default:
		// This should be impossible since we specify the primary key (request id)
		// in the WHERE clause of the update.
		return db.ErrMultipleUpdated
	}
}
