package cmd

import (
	"bytes"
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
)

// fakeProvider records the Provider calls the migrate command makes.
type fakeProvider struct {
	calls []string
}

func (f *fakeProvider) record(call string) {
	f.calls = append(f.calls, call)
}

func (f *fakeProvider) Up(context.Context) ([]*goose.MigrationResult, error) {
	f.record("Up")
	return nil, nil
}

func (f *fakeProvider) UpByOne(context.Context) (*goose.MigrationResult, error) {
	f.record("UpByOne")
	return &goose.MigrationResult{Source: &goose.Source{Version: 1}}, nil
}

func (f *fakeProvider) UpTo(_ context.Context, v int64) ([]*goose.MigrationResult, error) {
	f.record("UpTo " + strconv.FormatInt(v, 10))
	return nil, nil
}

func (f *fakeProvider) Down(context.Context) (*goose.MigrationResult, error) {
	f.record("Down")
	return &goose.MigrationResult{Source: &goose.Source{Version: 1}}, nil
}

func (f *fakeProvider) DownTo(_ context.Context, v int64) ([]*goose.MigrationResult, error) {
	f.record("DownTo " + strconv.FormatInt(v, 10))
	return nil, nil
}

func (f *fakeProvider) Status(context.Context) ([]*goose.MigrationStatus, error) {
	f.record("Status")
	return nil, nil
}

func (f *fakeProvider) GetDBVersion(context.Context) (int64, error) {
	f.record("GetDBVersion")
	return 7, nil
}

func TestRunProviderCommand(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		args      []string
		wantCalls []string
		wantErr   error
	}{
		{name: "up", command: "up", wantCalls: []string{"Up"}},
		{name: "up-by-one", command: "up-by-one", wantCalls: []string{"UpByOne"}},
		{name: "up-to", command: "up-to", args: []string{"5"}, wantCalls: []string{"UpTo 5"}},
		{name: "up-to without version", command: "up-to", wantErr: ErrMigrateVersionRequired},
		{name: "up-to with invalid version", command: "up-to", args: []string{"five"}, wantErr: strconv.ErrSyntax},
		{name: "down", command: "down", wantCalls: []string{"Down"}},
		{name: "down-to", command: "down-to", args: []string{"5"}, wantCalls: []string{"DownTo 5"}},
		{name: "down-to without version", command: "down-to", wantErr: ErrMigrateVersionRequired},
		{name: "redo", command: "redo", wantCalls: []string{"Down", "UpByOne"}},
		{name: "reset", command: "reset", wantCalls: []string{"DownTo 0"}},
		{name: "status", command: "status", wantCalls: []string{"Status"}},
		{name: "version", command: "version", wantCalls: []string{"GetDBVersion"}},
		{name: "create is not a provider command", command: "create", args: []string{"x", "sql"}, wantErr: ErrUnsupportedMigrateCommand},
		{name: "fix is not a provider command", command: "fix", wantErr: ErrUnsupportedMigrateCommand},
		{name: "unknown command", command: "sideways", wantErr: ErrUnsupportedMigrateCommand},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &fakeProvider{}

			err := runProviderCommand(context.Background(), p, tt.command, tt.args, &bytes.Buffer{})
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.wantCalls, p.calls)
		})
	}
}

func TestIsFileCommand(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{command: "create", want: true},
		{command: "fix", want: true},
		{command: "up", want: false},
		{command: "status", want: false},
		{command: "sideways", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			assert.Equal(t, tt.want, isFileCommand(tt.command))
		})
	}
}

func TestPrintStatus(t *testing.T) {
	out := &bytes.Buffer{}

	printStatus(out, []*goose.MigrationStatus{
		{Source: &goose.Source{Version: 6}, State: goose.StateApplied, AppliedAt: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)},
		{Source: &goose.Source{Version: 7}, State: goose.StatePending},
	})

	assert.Equal(t, "6\tapplied\t2026-11-18T12:00:00Z\n7\tpending\t-\n", out.String())
}
