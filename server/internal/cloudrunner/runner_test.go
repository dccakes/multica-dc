package cloudrunner

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/cloudrunner/provider"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type fakeLifecycleClient struct {
	mu sync.Mutex

	heartbeatCalls int
	claimCalls     int
	startCalls     int
	completeCalls  int
	failCalls      int
	heartbeatIDs   []string
	claimIDs       []string
	heartbeatErrs  []error
	claimErrs      []error
	claimTasks     []*Task
	heartbeatCh    chan string
	claimCh        chan string
}

func (f *fakeLifecycleClient) SendHeartbeat(_ context.Context, runtimeID string) error {
	f.mu.Lock()
	f.heartbeatIDs = append(f.heartbeatIDs, runtimeID)
	ch := f.heartbeatCh
	defer f.mu.Unlock()
	f.heartbeatCalls++
	if ch != nil {
		select {
		case ch <- runtimeID:
		default:
		}
	}
	if len(f.heartbeatErrs) == 0 {
		return nil
	}
	err := f.heartbeatErrs[0]
	f.heartbeatErrs = f.heartbeatErrs[1:]
	return err
}

func (f *fakeLifecycleClient) ClaimTask(_ context.Context, runtimeID string) (*Task, error) {
	f.mu.Lock()
	f.claimIDs = append(f.claimIDs, runtimeID)
	ch := f.claimCh
	defer f.mu.Unlock()
	f.claimCalls++
	if ch != nil {
		select {
		case ch <- runtimeID:
		default:
		}
	}
	if len(f.claimErrs) > 0 {
		err := f.claimErrs[0]
		f.claimErrs = f.claimErrs[1:]
		return nil, err
	}
	if len(f.claimTasks) == 0 {
		return nil, nil
	}
	task := f.claimTasks[0]
	f.claimTasks = f.claimTasks[1:]
	return task, nil
}

func (f *fakeLifecycleClient) snapshot() (heartbeats, claims int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.heartbeatCalls, f.claimCalls
}

func (f *fakeLifecycleClient) StartTask(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startCalls++
	return nil
}

func (f *fakeLifecycleClient) CompleteTask(_ context.Context, _, _, _, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.completeCalls++
	return nil
}

func (f *fakeLifecycleClient) FailTask(_ context.Context, _, _, _, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failCalls++
	return nil
}

func (f *fakeLifecycleClient) lifecycleSnapshot() (startCalls, completeCalls, failCalls int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.startCalls, f.completeCalls, f.failCalls
}

func (f *fakeLifecycleClient) snapshotRuntimeIDs() (heartbeatIDs []string, claimIDs []string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	heartbeatIDs = make([]string, len(f.heartbeatIDs))
	copy(heartbeatIDs, f.heartbeatIDs)
	claimIDs = make([]string, len(f.claimIDs))
	copy(claimIDs, f.claimIDs)
	return heartbeatIDs, claimIDs
}

type fakeProvider struct {
	mu sync.Mutex

	started bool
	stopped bool
	handled []Task

	startCalls int
	stopCalls  int
	handleErr  error
	handledCh  chan struct{}
	releaseCh  chan struct{}
	result     provider.Result
}

func (f *fakeProvider) Start(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.startCalls++
	f.started = true
	return nil
}

func (f *fakeProvider) Stop(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopCalls++
	f.stopped = true
	return nil
}

func (f *fakeProvider) ExecuteTask(_ context.Context, task Task) (provider.Result, error) {
	f.mu.Lock()
	f.handled = append(f.handled, task)
	ch := f.handledCh
	err := f.handleErr
	releaseCh := f.releaseCh
	result := f.result
	f.mu.Unlock()
	if ch != nil {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	if releaseCh != nil {
		<-releaseCh
	}
	if result.Output == "" && result.SessionID == "" && result.WorkDir == "" && result.Branch == "" && result.SandboxID == "" && result.SnapshotID == "" && result.SnapshotExpiresAt.IsZero() {
		result = provider.Result{Output: "ok", WorkDir: "/workspace", Branch: "main", SessionID: "sess_1"}
	}
	return result, err
}

func (f *fakeProvider) snapshot() (started, stopped bool, startCalls, stopCalls int, handled []Task) {
	f.mu.Lock()
	defer f.mu.Unlock()
	h := make([]Task, len(f.handled))
	copy(h, f.handled)
	return f.started, f.stopped, f.startCalls, f.stopCalls, h
}

type fakeSessionStoreForRunner struct {
	mu          sync.Mutex
	session     db.CloudRuntimeSession
	upsertCount int
	lastUpsert  db.UpsertCloudRuntimeSessionParams
}

func (f *fakeSessionStoreForRunner) GetCloudRuntimeSessionForIssue(context.Context, db.GetCloudRuntimeSessionForIssueParams) (db.CloudRuntimeSession, error) {
	return f.session, nil
}

func (f *fakeSessionStoreForRunner) UpsertCloudRuntimeSession(_ context.Context, arg db.UpsertCloudRuntimeSessionParams) (db.UpsertCloudRuntimeSessionRow, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upsertCount++
	f.lastUpsert = arg
	return db.UpsertCloudRuntimeSessionRow{}, nil
}

func (f *fakeSessionStoreForRunner) GetCloudRuntimeSessionForChat(context.Context, db.GetCloudRuntimeSessionForChatParams) (db.CloudRuntimeSession, error) {
	return db.CloudRuntimeSession{}, errors.New("not implemented in this test")
}

func (f *fakeSessionStoreForRunner) snapshot() (int, db.UpsertCloudRuntimeSessionParams) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.upsertCount, f.lastUpsert
}

func TestRunnerLifecycle_StartsLoopsAndDispatchesClaimedTask(t *testing.T) {
	client := &fakeLifecycleClient{
		claimTasks:  []*Task{{ID: "task-1", RuntimeID: "rt-1"}},
		heartbeatCh: make(chan string, 1),
	}
	provider := &fakeProvider{handledCh: make(chan struct{}, 1)}

	runner := NewRunner(Config{
		RuntimeID:         "rt-1",
		RuntimeIDs:        []string{"rt-1"},
		AuthToken:         "token",
		HeartbeatInterval: 10 * time.Millisecond,
		ClaimInterval:     10 * time.Millisecond,
	}, client, provider)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runner.Run(ctx)
	}()

	select {
	case <-client.heartbeatCh:
	case <-time.After(750 * time.Millisecond):
		t.Fatal("timed out waiting for first heartbeat")
	}

	select {
	case <-provider.handledCh:
	case <-time.After(750 * time.Millisecond):
		t.Fatal("timed out waiting for claimed task dispatch")
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context canceled", err)
		}
	case <-time.After(750 * time.Millisecond):
		t.Fatal("timed out waiting for runner shutdown")
	}

	heartbeats, claims := client.snapshot()
	if heartbeats < 1 {
		t.Fatalf("heartbeats = %d, want >= 1", heartbeats)
	}
	if claims < 1 {
		t.Fatalf("claims = %d, want >= 1", claims)
	}

	started, stopped, startCalls, stopCalls, handled := provider.snapshot()
	if !started || startCalls != 1 {
		t.Fatalf("provider start state invalid: started=%v startCalls=%d", started, startCalls)
	}
	if !stopped || stopCalls != 1 {
		t.Fatalf("provider stop state invalid: stopped=%v stopCalls=%d", stopped, stopCalls)
	}
	if len(handled) != 1 || handled[0].ID != "task-1" {
		t.Fatalf("handled tasks = %#v, want one task with ID task-1", handled)
	}
	startReports, completeReports, failReports := client.lifecycleSnapshot()
	if startReports < 1 {
		t.Fatalf("start reports = %d, want >= 1", startReports)
	}
	if completeReports < 1 {
		t.Fatalf("complete reports = %d, want >= 1", completeReports)
	}
	if failReports != 0 {
		t.Fatalf("fail reports = %d, want 0", failReports)
	}
}

func TestRunnerLifecycle_ContinuesAfterLoopErrors(t *testing.T) {
	client := &fakeLifecycleClient{
		heartbeatErrs: []error{errors.New("hb failed")},
		claimErrs:     []error{errors.New("claim failed")},
	}
	provider := &fakeProvider{}

	runner := NewRunner(Config{
		RuntimeID:         "rt-1",
		RuntimeIDs:        []string{"rt-1"},
		AuthToken:         "token",
		HeartbeatInterval: 5 * time.Millisecond,
		ClaimInterval:     5 * time.Millisecond,
	}, client, provider)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	err := runner.Run(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Run() error = %v, want deadline exceeded", err)
	}

	heartbeats, claims := client.snapshot()
	if heartbeats < 2 {
		t.Fatalf("heartbeats = %d, want >= 2 (loop should continue after first error)", heartbeats)
	}
	if claims < 2 {
		t.Fatalf("claims = %d, want >= 2 (loop should continue after first error)", claims)
	}
}

func TestRunnerHeartbeatLoop_HeartbeatsAllConfiguredRuntimeIDsEachTick(t *testing.T) {
	client := &fakeLifecycleClient{heartbeatCh: make(chan string, 16)}
	provider := &fakeProvider{}

	runner := NewRunner(Config{
		RuntimeIDs:        []string{"rt-1", "rt-2", "rt-3"},
		HeartbeatInterval: 5 * time.Millisecond,
		ClaimInterval:     time.Hour,
	}, client, provider)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runner.heartbeatLoop(ctx)

	seen := map[string]bool{}
	for len(seen) < 3 {
		select {
		case runtimeID := <-client.heartbeatCh:
			seen[runtimeID] = true
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting for heartbeat across all runtime IDs; seen=%v", seen)
		}
	}
}

func TestRunnerClaimLoop_RoundRobinAcrossRuntimeIDs(t *testing.T) {
	client := &fakeLifecycleClient{claimCh: make(chan string, 32)}
	provider := &fakeProvider{}

	runner := NewRunner(Config{
		RuntimeIDs:        []string{"rt-1", "rt-2", "rt-3"},
		HeartbeatInterval: time.Hour,
		ClaimInterval:     2 * time.Millisecond,
	}, client, provider)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runner.claimLoop(ctx)

	want := []string{"rt-1", "rt-2", "rt-3", "rt-1", "rt-2", "rt-3"}
	got := make([]string, 0, len(want))
	for len(got) < len(want) {
		select {
		case runtimeID := <-client.claimCh:
			got = append(got, runtimeID)
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting for claim calls; got=%v", got)
		}
	}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("claim runtime order mismatch at %d: got=%v want=%v", i, got, want)
		}
	}
}

func TestRunnerClaimLoop_IncludesResumeSnapshotFromSessionService(t *testing.T) {
	client := &fakeLifecycleClient{
		claimTasks: []*Task{{
			ID:        "task-1",
			RuntimeID: "11111111-1111-1111-1111-111111111111",
			AgentID:   "22222222-2222-2222-2222-222222222222",
			IssueID:   "33333333-3333-3333-3333-333333333333",
		}},
	}
	p := &fakeProvider{handledCh: make(chan struct{}, 1)}
	session := NewSessionService(&fakeSessionStoreForRunner{
		session: db.CloudRuntimeSession{
			LastSnapshotID:     pgtype.Text{String: "snap_resume", Valid: true},
			LastWorkdir:        pgtype.Text{String: "/workspace", Valid: true},
			LastBranch:         pgtype.Text{String: "main", Valid: true},
			LastCodexSessionID: pgtype.Text{String: "codex_1", Valid: true},
		},
	})

	runner := NewRunner(Config{
		RuntimeIDs:        []string{"rt-1"},
		AuthToken:         "token",
		HeartbeatInterval: time.Hour,
		ClaimInterval:     5 * time.Millisecond,
	}, client, p).WithSessionService(session)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runner.claimLoop(ctx)

	select {
	case <-p.handledCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for handled task")
	}
	cancel()

	_, _, _, _, handled := p.snapshot()
	if len(handled) == 0 {
		t.Fatal("expected handled task")
	}
	got := handled[0]
	if got.ResumeSnapshotID != "snap_resume" || got.ResumeSource != "resume" {
		t.Fatalf("resume fields missing from task: %#v", got)
	}
	if got.PriorWorkDir != "/workspace" {
		t.Fatalf("PriorWorkDir = %q, want /workspace", got.PriorWorkDir)
	}
	if got.PriorSessionID != "codex_1" {
		t.Fatalf("PriorSessionID = %q, want codex_1", got.PriorSessionID)
	}
}

func TestRunnerClaimLoop_PersistsTransitionAndHeartbeatCheckpoints(t *testing.T) {
	sessionExpiry := time.Now().UTC().Add(2 * time.Hour)
	client := &fakeLifecycleClient{
		claimTasks: []*Task{{
			ID:        "task-1",
			RuntimeID: "rt-1",
			AgentID:   "22222222-2222-2222-2222-222222222222",
			IssueID:   "33333333-3333-3333-3333-333333333333",
		}},
	}
	provider := &fakeProvider{
		handledCh: make(chan struct{}, 1),
		releaseCh: make(chan struct{}),
		result: provider.Result{
			Output:            "done",
			WorkDir:           "/workspace",
			Branch:            "main",
			SessionID:         "codex_1",
			SandboxID:         "sbx_1",
			SnapshotID:        "snap_new",
			SnapshotExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		},
	}
	sessionStore := &fakeSessionStoreForRunner{
		session: db.CloudRuntimeSession{
			LastSnapshotID:     pgtype.Text{String: "snap_resume", Valid: true},
			SnapshotExpiresAt:  pgtype.Timestamptz{Time: sessionExpiry, Valid: true},
			LastWorkdir:        pgtype.Text{String: "/workspace", Valid: true},
			LastBranch:         pgtype.Text{String: "feature/a", Valid: true},
			LastCodexSessionID: pgtype.Text{String: "codex_123", Valid: true},
		},
	}
	session := NewSessionService(sessionStore)

	runner := NewRunner(Config{
		RuntimeID:         "rt-1",
		RuntimeIDs:        []string{"rt-1"},
		AuthToken:         "token",
		HeartbeatInterval: 5 * time.Millisecond,
		ClaimInterval:     5 * time.Millisecond,
	}, client, provider).WithSessionService(session)
	runner.checkpointInterval = 5 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runner.Run(ctx)
	}()

	select {
	case <-provider.handledCh:
	case <-time.After(750 * time.Millisecond):
		t.Fatal("timed out waiting for task execution to start")
	}

	waitUntil := func(cond func() bool, timeout time.Duration) bool {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			if cond() {
				return true
			}
			time.Sleep(5 * time.Millisecond)
		}
		return cond()
	}

	if !waitUntil(func() bool {
		count, _ := sessionStore.snapshot()
		return count >= 2
	}, 400*time.Millisecond) {
		count, _ := sessionStore.snapshot()
		t.Fatalf("checkpoint upserts = %d, want >= 2 before task release", count)
	}

	close(provider.releaseCh)

	if !waitUntil(func() bool {
		count, _ := sessionStore.snapshot()
		return count >= 3
	}, 400*time.Millisecond) {
		count, _ := sessionStore.snapshot()
		t.Fatalf("checkpoint upserts = %d, want >= 3 after task completion", count)
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Run() error = %v, want context canceled", err)
		}
	case <-time.After(750 * time.Millisecond):
		t.Fatal("timed out waiting for runner shutdown")
	}
}

func TestRunnerClaimLoop_ReportsFailWhenProviderExecutionErrors(t *testing.T) {
	client := &fakeLifecycleClient{
		claimTasks: []*Task{{ID: "task-err", RuntimeID: "rt-1"}},
	}
	p := &fakeProvider{
		handleErr: errors.New("provider failed"),
		handledCh: make(chan struct{}, 1),
	}

	runner := NewRunner(Config{
		RuntimeIDs:        []string{"rt-1"},
		AuthToken:         "token",
		HeartbeatInterval: time.Hour,
		ClaimInterval:     5 * time.Millisecond,
	}, client, p)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go runner.claimLoop(ctx)

	select {
	case <-p.handledCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for provider execution attempt")
	}
	deadline := time.Now().Add(300 * time.Millisecond)
	for {
		_, _, failReports := client.lifecycleSnapshot()
		if failReports >= 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("fail reports = %d, want >= 1", failReports)
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
}
