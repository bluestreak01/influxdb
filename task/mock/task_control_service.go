package mock

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/snowflake"
	"github.com/influxdata/influxdb/task/backend"
	cron "gopkg.in/robfig/cron.v2"
)

var idgen = snowflake.NewDefaultIDGenerator()

// TaskControlService is a mock implementation of TaskControlService (used by NewScheduler).
type TaskControlService struct {
	mu sync.Mutex
	// Map of stringified task ID to last ID used for run.
	runs map[influxdb.ID]map[influxdb.ID]*influxdb.Run

	// Map of stringified, concatenated task and platform ID, to runs that have been created.
	created map[string]backend.QueuedRun

	// Map of stringified task ID to task meta.
	tasks      map[influxdb.ID]*influxdb.Task
	manualRuns []*influxdb.Run
	// Map of task ID to total number of runs created for that task.
	totalRunsCreated map[influxdb.ID]int
	finishedRuns     map[influxdb.ID]*influxdb.Run
}

var _ backend.TaskControlService = (*TaskControlService)(nil)

func NewTaskControlService() *TaskControlService {
	return &TaskControlService{
		runs:             make(map[influxdb.ID]map[influxdb.ID]*influxdb.Run),
		finishedRuns:     make(map[influxdb.ID]*influxdb.Run),
		tasks:            make(map[influxdb.ID]*influxdb.Task),
		created:          make(map[string]backend.QueuedRun),
		totalRunsCreated: make(map[influxdb.ID]int),
	}
}

// SetTask sets the task.
// SetTask must be called before CreateNextRun, for a given task ID.
func (d *TaskControlService) SetTask(task *influxdb.Task) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.tasks[task.ID] = task
}

func (d *TaskControlService) SetManualRuns(runs []*influxdb.Run) {
	d.manualRuns = runs
}

// CreateNextRun creates the next run for the given task.
// Refer to the documentation for SetTaskPeriod to understand how the times are determined.
func (d *TaskControlService) CreateNextRun(ctx context.Context, taskID influxdb.ID, now int64) (backend.RunCreation, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !taskID.Valid() {
		return backend.RunCreation{}, errors.New("invalid task id")
	}
	tid := taskID

	task, ok := d.tasks[tid]
	if !ok {
		panic(fmt.Sprintf("meta not set for task with ID %s", tid))
	}

	if len(d.manualRuns) != 0 {
		run := d.manualRuns[0]
		d.manualRuns = d.manualRuns[1:]
		runs, ok := d.runs[tid]
		if !ok {
			runs = make(map[influxdb.ID]*influxdb.Run)
		}
		runs[run.ID] = run
		d.runs[task.ID] = runs
		next, _ := d.nextDueRun(ctx, taskID)
		rc := backend.RunCreation{
			Created: backend.QueuedRun{
				TaskID: task.ID,
				RunID:  run.ID,
				Now:    run.ScheduledFor.Unix(),
			},
			NextDue:  next,
			HasQueue: len(d.manualRuns) != 0,
		}
		d.created[tid.String()+rc.Created.RunID.String()] = rc.Created
		d.totalRunsCreated[taskID]++
		return rc, nil
	}

	rc, err := d.createNextRun(task, now)
	if err != nil {
		return backend.RunCreation{}, err
	}
	rc.Created.TaskID = taskID
	d.created[tid.String()+rc.Created.RunID.String()] = rc.Created
	d.totalRunsCreated[taskID]++
	return rc, nil
}

func (t *TaskControlService) createNextRun(task *influxdb.Task, now int64) (backend.RunCreation, error) {
	sch, err := cron.Parse(task.EffectiveCron())
	if err != nil {
		return backend.RunCreation{}, err
	}
	latest := int64(0)
	lt, err := time.Parse(time.RFC3339, task.LatestCompleted)
	if err == nil {
		latest = lt.Unix()
	}
	for _, r := range t.runs[task.ID] {
		if r.ScheduledFor.Unix() > latest {
			latest = r.ScheduledFor.Unix()
		}

	}

	nextScheduled := sch.Next(time.Unix(latest, 0))
	nextScheduledUnix := nextScheduled.Unix()
	offset := int64(0)
	if task.Offset != "" {
		toff, err := time.ParseDuration(task.Offset)
		if err == nil {
			offset = toff.Nanoseconds()
		}
	}
	if dueAt := nextScheduledUnix + int64(offset); dueAt > now {
		return backend.RunCreation{}, influxdb.ErrRunNotDueYet(dueAt)
	}

	runID := idgen.ID()
	runs, ok := t.runs[task.ID]
	if !ok {
		runs = make(map[influxdb.ID]*influxdb.Run)
	}
	runs[runID] = &influxdb.Run{
		ID:           runID,
		ScheduledFor: nextScheduled,
	}
	t.runs[task.ID] = runs

	return backend.RunCreation{
		Created: backend.QueuedRun{
			RunID: runID,
			Now:   nextScheduledUnix,
		},
		NextDue:  sch.Next(nextScheduled).Unix() + offset,
		HasQueue: false,
	}, nil
}

func (t *TaskControlService) CreateRun(_ context.Context, taskID influxdb.ID, scheduledFor time.Time) (*influxdb.Run, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	runID := idgen.ID()
	runs, ok := t.runs[taskID]
	if !ok {
		runs = make(map[influxdb.ID]*influxdb.Run)
	}
	runs[runID] = &influxdb.Run{
		ID:           runID,
		ScheduledFor: scheduledFor,
	}
	t.runs[taskID] = runs
	return runs[runID], nil
}

func (t *TaskControlService) StartManualRun(_ context.Context, taskID, runID influxdb.ID) (*influxdb.Run, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var run *influxdb.Run
	for i, r := range t.manualRuns {
		if r.ID == runID {
			run = r
			t.manualRuns = append(t.manualRuns[:i], t.manualRuns[i+1:]...)
		}
	}
	if run == nil {
		return nil, influxdb.ErrRunNotFound
	}
	return run, nil
}

func (d *TaskControlService) FinishRun(_ context.Context, taskID, runID influxdb.ID) (*influxdb.Run, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	tid := taskID
	rid := runID
	r := d.runs[tid][rid]
	delete(d.runs[tid], rid)
	t := d.tasks[tid]
	schedFor := r.ScheduledFor.Format(time.RFC3339)

	if t.LatestCompleted != "" {
		var latest time.Time
		latest, err := time.Parse(time.RFC3339, t.LatestCompleted)
		if err != nil {
			return nil, err
		}

		if r.ScheduledFor.After(latest) {
			t.LatestCompleted = schedFor
		}
	}
	d.finishedRuns[rid] = r
	delete(d.created, tid.String()+rid.String())
	return r, nil
}

func (t *TaskControlService) CurrentlyRunning(ctx context.Context, taskID influxdb.ID) ([]*influxdb.Run, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	rtn := []*influxdb.Run{}
	for _, run := range t.runs[taskID] {
		rtn = append(rtn, run)
	}
	return rtn, nil
}

func (t *TaskControlService) ManualRuns(ctx context.Context, taskID influxdb.ID) ([]*influxdb.Run, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.manualRuns != nil {
		return t.manualRuns, nil
	}
	return []*influxdb.Run{}, nil
}

// NextDueRun returns the Unix timestamp of when the next call to CreateNextRun will be ready.
// The returned timestamp reflects the task's offset, so it does not necessarily exactly match the schedule time.
func (d *TaskControlService) NextDueRun(ctx context.Context, taskID influxdb.ID) (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.nextDueRun(ctx, taskID)
}

func (d *TaskControlService) nextDueRun(ctx context.Context, taskID influxdb.ID) (int64, error) {
	task := d.tasks[taskID]
	sch, err := cron.Parse(task.EffectiveCron())
	if err != nil {
		return 0, err
	}
	latest := int64(0)
	lt, err := time.Parse(time.RFC3339, task.LatestCompleted)
	if err == nil {
		latest = lt.Unix()
	}

	for _, r := range d.runs[task.ID] {
		if r.ScheduledFor.Unix() > latest {
			latest = r.ScheduledFor.Unix()
		}
	}

	nextScheduled := sch.Next(time.Unix(latest, 0))
	nextScheduledUnix := nextScheduled.Unix()
	offset := int64(0)
	if task.Offset != "" {
		toff, err := time.ParseDuration(task.Offset)
		if err == nil {
			offset = toff.Nanoseconds()
		}
	}

	return nextScheduledUnix + int64(offset), nil
}

// UpdateRunState sets the run state at the respective time.
func (d *TaskControlService) UpdateRunState(ctx context.Context, taskID, runID influxdb.ID, when time.Time, state backend.RunStatus) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	run, ok := d.runs[taskID][runID]
	if !ok {
		panic("run state called without a run")
	}
	switch state {
	case backend.RunStarted:
		run.StartedAt = when
	case backend.RunSuccess, backend.RunFail, backend.RunCanceled:
		run.FinishedAt = when
	case backend.RunScheduled:
		// nothing
	default:
		panic("invalid status")
	}
	run.Status = state.String()
	return nil
}

// AddRunLog adds a log line to the run.
func (d *TaskControlService) AddRunLog(ctx context.Context, taskID, runID influxdb.ID, when time.Time, log string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	run := d.runs[taskID][runID]
	if run == nil {
		panic("cannot add a log to a non existent run")
	}
	run.Log = append(run.Log, influxdb.Log{RunID: runID, Time: when.Format(time.RFC3339Nano), Message: log})
	return nil
}

func (d *TaskControlService) CreatedFor(taskID influxdb.ID) []backend.QueuedRun {
	d.mu.Lock()
	defer d.mu.Unlock()

	var qrs []backend.QueuedRun
	for _, qr := range d.created {
		if qr.TaskID == taskID {
			qrs = append(qrs, qr)
		}
	}

	return qrs
}

// TotalRunsCreatedForTask returns the number of runs created for taskID.
func (d *TaskControlService) TotalRunsCreatedForTask(taskID influxdb.ID) int {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.totalRunsCreated[taskID]
}

// PollForNumberCreated blocks for a small amount of time waiting for exactly the given count of created and unfinished runs for the given task ID.
// If the expected number isn't found in time, it returns an error.
//
// Because the scheduler and executor do a lot of state changes asynchronously, this is useful in test.
func (d *TaskControlService) PollForNumberCreated(taskID influxdb.ID, count int) ([]backend.QueuedRun, error) {
	const numAttempts = 50
	actualCount := 0
	var created []backend.QueuedRun
	for i := 0; i < numAttempts; i++ {
		time.Sleep(2 * time.Millisecond) // we sleep even on first so it becomes more likely that we catch when too many are produced.
		created = d.CreatedFor(taskID)
		actualCount = len(created)
		if actualCount == count {
			return created, nil
		}
	}
	return created, fmt.Errorf("did not see count of %d created run(s) for task with ID %s in time, instead saw %d", count, taskID, actualCount) // we return created anyways, to make it easier to debug
}

func (d *TaskControlService) FinishedRun(runID influxdb.ID) *influxdb.Run {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.finishedRuns[runID]
}

func (d *TaskControlService) FinishedRuns() []*influxdb.Run {
	rtn := []*influxdb.Run{}
	for _, run := range d.finishedRuns {
		rtn = append(rtn, run)
	}

	sort.Slice(rtn, func(i, j int) bool { return rtn[i].ScheduledFor.Before(rtn[j].ScheduledFor) })
	return rtn
}
