package http

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/http/metric"
	httpmock "github.com/influxdata/influxdb/http/mock"
	"github.com/influxdata/influxdb/mock"
	influxtesting "github.com/influxdata/influxdb/testing"
	"go.uber.org/zap/zaptest"
)

func TestWriteService_Write(t *testing.T) {
	type args struct {
		org    influxdb.ID
		bucket influxdb.ID
		r      io.Reader
	}
	tests := []struct {
		name    string
		args    args
		status  int
		want    string
		wantErr bool
	}{
		{
			args: args{
				org:    1,
				bucket: 2,
				r:      strings.NewReader("m,t1=v1 f1=2"),
			},
			status: http.StatusNoContent,
			want:   "m,t1=v1 f1=2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var org, bucket *influxdb.ID
			var lp []byte
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				org, _ = influxdb.IDFromString(r.URL.Query().Get("org"))
				bucket, _ = influxdb.IDFromString(r.URL.Query().Get("bucket"))
				defer r.Body.Close()
				in, _ := gzip.NewReader(r.Body)
				defer in.Close()
				lp, _ = ioutil.ReadAll(in)
				w.WriteHeader(tt.status)
			}))
			s := &WriteService{
				Addr: ts.URL,
			}
			if err := s.Write(context.Background(), tt.args.org, tt.args.bucket, tt.args.r); (err != nil) != tt.wantErr {
				t.Errorf("WriteService.Write() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got, want := *org, tt.args.org; got != want {
				t.Errorf("WriteService.Write() org = %v, want %v", got, want)
			}

			if got, want := *bucket, tt.args.bucket; got != want {
				t.Errorf("WriteService.Write() bucket = %v, want %v", got, want)
			}

			if got, want := string(lp), tt.want; got != want {
				t.Errorf("WriteService.Write() = %v, want %v", got, want)
			}
		})
	}
}

func TestWriteHandler_handleWrite(t *testing.T) {
	// state is the internal state of org and bucket services
	type state struct {
		org       *influxdb.Organization // org to return in org service
		orgErr    error                  // err to return in org servce
		bucket    *influxdb.Bucket       // bucket to return in bucket service
		bucketErr error                  // err to return in bucket service
		writeErr  error                  // err to return from the points writer
	}

	// want is the expected output of the HTTP endpoint
	type wants struct {
		body string
		code int
	}

	// request is sent to the HTTP endpoint
	type request struct {
		auth   influxdb.Authorizer
		org    string
		bucket string
		body   string
	}

	tests := []struct {
		name    string
		request request
		state   state
		wants   wants
	}{
		{
			name: "simple body is accepted",
			request: request{
				org:    "043e0780ee2b1000",
				bucket: "04504b356e23b000",
				body:   "m1,t1=v1 f1=1",
				auth:   bucketWritePermission("043e0780ee2b1000", "04504b356e23b000"),
			},
			state: state{
				org:    testOrg("043e0780ee2b1000"),
				bucket: testBucket("043e0780ee2b1000", "04504b356e23b000"),
			},
			wants: wants{
				code: 204,
			},
		},
		{
			name: "points writer error is an internal error",
			request: request{
				org:    "043e0780ee2b1000",
				bucket: "04504b356e23b000",
				body:   "m1,t1=v1 f1=1",
				auth:   bucketWritePermission("043e0780ee2b1000", "04504b356e23b000"),
			},
			state: state{
				org:      testOrg("043e0780ee2b1000"),
				bucket:   testBucket("043e0780ee2b1000", "04504b356e23b000"),
				writeErr: fmt.Errorf("error"),
			},
			wants: wants{
				code: 500,
				body: `{"code":"internal error","message":"unexpected error writing points to database: error"}`,
			},
		},
		{
			name: "empty request body returns 400 error",
			request: request{
				org:    "043e0780ee2b1000",
				bucket: "04504b356e23b000",
				auth:   bucketWritePermission("043e0780ee2b1000", "04504b356e23b000"),
			},
			state: state{
				org:    testOrg("043e0780ee2b1000"),
				bucket: testBucket("043e0780ee2b1000", "04504b356e23b000"),
			},
			wants: wants{
				code: 400,
				body: `{"code":"invalid","message":"writing requires points"}`,
			},
		},
		{
			name: "org error returns 404 error",
			request: request{
				org:    "043e0780ee2b1000",
				bucket: "04504b356e23b000",
				body:   "m1,t1=v1 f1=1",
				auth:   bucketWritePermission("043e0780ee2b1000", "04504b356e23b000"),
			},
			state: state{
				orgErr: &influxdb.Error{Code: influxdb.ENotFound, Msg: "not found"},
			},
			wants: wants{
				code: 404,
				body: `{"code":"not found","message":"not found"}`,
			},
		},
		{
			name: "bucket error returns 404 error",
			request: request{
				org:    "043e0780ee2b1000",
				bucket: "04504b356e23b000",
				body:   "m1,t1=v1 f1=1",
				auth:   bucketWritePermission("043e0780ee2b1000", "04504b356e23b000"),
			},
			state: state{
				org:       testOrg("043e0780ee2b1000"),
				bucketErr: &influxdb.Error{Code: influxdb.ENotFound, Msg: "not found"},
			},
			wants: wants{
				code: 404,
				body: `{"code":"not found","message":"not found"}`,
			},
		},
		{
			name: "500 when bucket service returns internal error",
			request: request{
				org:    "043e0780ee2b1000",
				bucket: "04504b356e23b000",
				auth:   bucketWritePermission("043e0780ee2b1000", "04504b356e23b000"),
			},
			state: state{
				org:       testOrg("043e0780ee2b1000"),
				bucketErr: &influxdb.Error{Code: influxdb.EInternal, Msg: "internal error"},
			},
			wants: wants{
				code: 500,
				body: `{"code":"internal error","message":"internal error"}`,
			},
		},
		{
			name: "invalid line protocol returns 400",
			request: request{
				org:    "043e0780ee2b1000",
				bucket: "04504b356e23b000",
				auth:   bucketWritePermission("043e0780ee2b1000", "04504b356e23b000"),
				body:   "invalid",
			},
			state: state{
				org:    testOrg("043e0780ee2b1000"),
				bucket: testBucket("043e0780ee2b1000", "04504b356e23b000"),
			},
			wants: wants{
				code: 400,
				body: `{"code":"invalid","message":"unable to parse 'invalid': missing fields"}`,
			},
		},
		{
			name: "forbidden to write with insufficient permission",
			request: request{
				org:    "043e0780ee2b1000",
				bucket: "04504b356e23b000",
				body:   "m1,t1=v1 f1=1",
				auth:   bucketWritePermission("043e0780ee2b1000", "000000000000000a"),
			},
			state: state{
				org:    testOrg("043e0780ee2b1000"),
				bucket: testBucket("043e0780ee2b1000", "04504b356e23b000"),
			},
			wants: wants{
				code: 403,
				body: `{"code":"forbidden","message":"insufficient permissions for write"}`,
			},
		},
		{
			// authorization extraction happens in a different middleware.
			name: "no authorizer is an internal error",
			request: request{
				org:    "043e0780ee2b1000",
				bucket: "04504b356e23b000",
			},
			state: state{
				org:    testOrg("043e0780ee2b1000"),
				bucket: testBucket("043e0780ee2b1000", "04504b356e23b000"),
			},
			wants: wants{
				code: 500,
				body: `{"code":"internal error","message":"authorizer not found on context"}`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgs := mock.NewOrganizationService()
			orgs.FindOrganizationF = func(ctx context.Context, filter influxdb.OrganizationFilter) (*influxdb.Organization, error) {
				return tt.state.org, tt.state.orgErr
			}
			buckets := mock.NewBucketService()
			buckets.FindBucketFn = func(context.Context, influxdb.BucketFilter) (*influxdb.Bucket, error) {
				return tt.state.bucket, tt.state.bucketErr
			}

			b := &APIBackend{
				HTTPErrorHandler:    DefaultErrorHandler,
				Logger:              zaptest.NewLogger(t),
				OrganizationService: orgs,
				BucketService:       buckets,
				PointsWriter:        &mock.PointsWriter{Err: tt.state.writeErr},
				WriteEventRecorder:  &metric.NopEventRecorder{},
			}
			writeHandler := NewWriteHandler(NewWriteBackend(b))
			handler := httpmock.NewAuthMiddlewareHandler(writeHandler, tt.request.auth)

			r := httptest.NewRequest(
				"POST",
				"http://localhost:9999/api/v2/write",
				strings.NewReader(tt.request.body),
			)

			params := r.URL.Query()
			params.Set("org", tt.request.org)
			params.Set("bucket", tt.request.bucket)
			r.URL.RawQuery = params.Encode()

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if got, want := w.Code, tt.wants.code; got != want {
				t.Errorf("unexpected status code: got %d want %d", got, want)
			}

			if got, want := w.Body.String(), tt.wants.body; got != want {
				t.Errorf("unexpected body: got %s want %s", got, want)
			}
		})
	}
}

var DefaultErrorHandler = ErrorHandler(0)

func bucketWritePermission(org, bucket string) *influxdb.Authorization {
	oid := influxtesting.MustIDBase16(org)
	bid := influxtesting.MustIDBase16(bucket)
	return &influxdb.Authorization{
		OrgID:  oid,
		Status: influxdb.Active,
		Permissions: []influxdb.Permission{
			{
				Action: influxdb.WriteAction,
				Resource: influxdb.Resource{
					Type:  influxdb.BucketsResourceType,
					OrgID: &oid,
					ID:    &bid,
				},
			},
		},
	}
}

func testOrg(org string) *influxdb.Organization {
	oid := influxtesting.MustIDBase16(org)
	return &influxdb.Organization{
		ID: oid,
	}
}

func testBucket(org, bucket string) *influxdb.Bucket {
	oid := influxtesting.MustIDBase16(org)
	bid := influxtesting.MustIDBase16(bucket)

	return &influxdb.Bucket{
		ID:    bid,
		OrgID: oid,
	}
}
