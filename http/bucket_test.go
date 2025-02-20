package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	platform "github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/inmem"
	"github.com/influxdata/influxdb/kv"
	"github.com/influxdata/influxdb/mock"
	platformtesting "github.com/influxdata/influxdb/testing"
	"github.com/julienschmidt/httprouter"
	"go.uber.org/zap"
)

// NewMockBucketBackend returns a BucketBackend with mock services.
func NewMockBucketBackend() *BucketBackend {
	return &BucketBackend{
		Logger: zap.NewNop().With(zap.String("handler", "bucket")),

		BucketService:              mock.NewBucketService(),
		BucketOperationLogService:  mock.NewBucketOperationLogService(),
		UserResourceMappingService: mock.NewUserResourceMappingService(),
		LabelService:               mock.NewLabelService(),
		UserService:                mock.NewUserService(),
		OrganizationService:        mock.NewOrganizationService(),
	}
}

func TestService_handleGetBuckets(t *testing.T) {
	type fields struct {
		BucketService platform.BucketService
		LabelService  platform.LabelService
	}
	type args struct {
		queryParams map[string][]string
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "get all buckets",
			fields: fields{
				&mock.BucketService{
					FindBucketsFn: func(ctx context.Context, filter platform.BucketFilter, opts ...platform.FindOptions) ([]*platform.Bucket, int, error) {
						return []*platform.Bucket{
							{
								ID:              platformtesting.MustIDBase16("0b501e7e557ab1ed"),
								Name:            "hello",
								OrgID:           platformtesting.MustIDBase16("50f7ba1150f7ba11"),
								RetentionPeriod: 2 * time.Second,
							},
							{
								ID:              platformtesting.MustIDBase16("c0175f0077a77005"),
								Name:            "example",
								OrgID:           platformtesting.MustIDBase16("7e55e118dbabb1ed"),
								RetentionPeriod: 24 * time.Hour,
							},
						}, 2, nil
					},
				},
				&mock.LabelService{
					FindResourceLabelsFn: func(ctx context.Context, f platform.LabelMappingFilter) ([]*platform.Label, error) {
						labels := []*platform.Label{
							{
								ID:   platformtesting.MustIDBase16("fc3dc670a4be9b9a"),
								Name: "label",
								Properties: map[string]string{
									"color": "fff000",
								},
							},
						}
						return labels, nil
					},
				},
			},
			args: args{
				map[string][]string{
					"limit": {"1"},
				},
			},
			wants: wants{
				statusCode:  http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "self": "/api/v2/buckets?descending=false&limit=1&offset=0",
    "next": "/api/v2/buckets?descending=false&limit=1&offset=1"
  },
  "buckets": [
    {
      "links": {
        "org": "/api/v2/orgs/50f7ba1150f7ba11",
        "self": "/api/v2/buckets/0b501e7e557ab1ed",
        "logs": "/api/v2/buckets/0b501e7e557ab1ed/logs",
        "labels": "/api/v2/buckets/0b501e7e557ab1ed/labels",
        "owners": "/api/v2/buckets/0b501e7e557ab1ed/owners",
        "members": "/api/v2/buckets/0b501e7e557ab1ed/members",
        "write": "/api/v2/write?org=50f7ba1150f7ba11&bucket=0b501e7e557ab1ed"
	  },
	  "createdAt": "0001-01-01T00:00:00Z",
	  "updatedAt": "0001-01-01T00:00:00Z",
      "id": "0b501e7e557ab1ed",
      "orgID": "50f7ba1150f7ba11",
			"type": "user",
      "name": "hello",
      "retentionRules": [{"type": "expire", "everySeconds": 2}],
			"labels": [
        {
          "id": "fc3dc670a4be9b9a",
          "name": "label",
          "properties": {
            "color": "fff000"
          }
        }
      ]
    },
    {
      "links": {
        "org": "/api/v2/orgs/7e55e118dbabb1ed",
        "self": "/api/v2/buckets/c0175f0077a77005",
        "logs": "/api/v2/buckets/c0175f0077a77005/logs",
        "labels": "/api/v2/buckets/c0175f0077a77005/labels",
        "members": "/api/v2/buckets/c0175f0077a77005/members",
        "owners": "/api/v2/buckets/c0175f0077a77005/owners",
        "write": "/api/v2/write?org=7e55e118dbabb1ed&bucket=c0175f0077a77005"
	  },
	  "createdAt": "0001-01-01T00:00:00Z",
	  "updatedAt": "0001-01-01T00:00:00Z",
      "id": "c0175f0077a77005",
      "orgID": "7e55e118dbabb1ed",
			"type": "user",
      "name": "example",
      "retentionRules": [{"type": "expire", "everySeconds": 86400}],
      "labels": [
        {
          "id": "fc3dc670a4be9b9a",
          "name": "label",
          "properties": {
            "color": "fff000"
          }
        }
      ]
    }
  ]
}
`,
			},
		},
		{
			name: "get all buckets when there are none",
			fields: fields{
				&mock.BucketService{
					FindBucketsFn: func(ctx context.Context, filter platform.BucketFilter, opts ...platform.FindOptions) ([]*platform.Bucket, int, error) {
						return []*platform.Bucket{}, 0, nil
					},
				},
				&mock.LabelService{},
			},
			args: args{
				map[string][]string{
					"limit": {"1"},
				},
			},
			wants: wants{
				statusCode:  http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "self": "/api/v2/buckets?descending=false&limit=1&offset=0"
  },
  "buckets": []
}`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucketBackend := NewMockBucketBackend()
			bucketBackend.BucketService = tt.fields.BucketService
			bucketBackend.LabelService = tt.fields.LabelService
			h := NewBucketHandler(bucketBackend)

			r := httptest.NewRequest("GET", "http://any.url", nil)

			qp := r.URL.Query()
			for k, vs := range tt.args.queryParams {
				for _, v := range vs {
					qp.Add(k, v)
				}
			}
			r.URL.RawQuery = qp.Encode()

			w := httptest.NewRecorder()

			h.handleGetBuckets(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handleGetBuckets() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handleGetBuckets() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil || tt.wants.body != "" && !eq {
				t.Errorf("%q. handleGetBuckets() = ***%v***", tt.name, diff)
			}
		})
	}
}

func TestService_handleGetBucket(t *testing.T) {
	type fields struct {
		BucketService platform.BucketService
	}
	type args struct {
		id string
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "get a bucket by id",
			fields: fields{
				&mock.BucketService{
					FindBucketByIDFn: func(ctx context.Context, id platform.ID) (*platform.Bucket, error) {
						if id == platformtesting.MustIDBase16("020f755c3c082000") {
							return &platform.Bucket{
								ID:              platformtesting.MustIDBase16("020f755c3c082000"),
								OrgID:           platformtesting.MustIDBase16("020f755c3c082000"),
								Name:            "hello",
								RetentionPeriod: 30 * time.Second,
							}, nil
						}

						return nil, fmt.Errorf("not found")
					},
				},
			},
			args: args{
				id: "020f755c3c082000",
			},
			wants: wants{
				statusCode:  http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body: `
		{
		  "links": {
		    "org": "/api/v2/orgs/020f755c3c082000",
		    "self": "/api/v2/buckets/020f755c3c082000",
		    "logs": "/api/v2/buckets/020f755c3c082000/logs",
		    "labels": "/api/v2/buckets/020f755c3c082000/labels",
		    "members": "/api/v2/buckets/020f755c3c082000/members",
		    "owners": "/api/v2/buckets/020f755c3c082000/owners",
		    "write": "/api/v2/write?org=020f755c3c082000&bucket=020f755c3c082000"
		  },
		  "createdAt": "0001-01-01T00:00:00Z",
		  "updatedAt": "0001-01-01T00:00:00Z",
		  "id": "020f755c3c082000",
		  "orgID": "020f755c3c082000",
			"type": "user",
		  "name": "hello",
		  "retentionRules": [{"type": "expire", "everySeconds": 30}],
      "labels": []
		}
		`,
			},
		},
		{
			name: "not found",
			fields: fields{
				&mock.BucketService{
					FindBucketByIDFn: func(ctx context.Context, id platform.ID) (*platform.Bucket, error) {
						return nil, &platform.Error{
							Code: platform.ENotFound,
							Msg:  "bucket not found",
						}
					},
				},
			},
			args: args{
				id: "020f755c3c082000",
			},
			wants: wants{
				statusCode: http.StatusNotFound,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucketBackend := NewMockBucketBackend()
			bucketBackend.HTTPErrorHandler = ErrorHandler(0)
			bucketBackend.BucketService = tt.fields.BucketService
			h := NewBucketHandler(bucketBackend)

			r := httptest.NewRequest("GET", "http://any.url", nil)

			r = r.WithContext(context.WithValue(
				context.Background(),
				httprouter.ParamsKey,
				httprouter.Params{
					{
						Key:   "id",
						Value: tt.args.id,
					},
				}))

			w := httptest.NewRecorder()

			h.handleGetBucket(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)
			t.Logf(res.Header.Get("X-Influx-Error"))

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handleGetBucket() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handleGetBucket() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if tt.wants.body != "" {
				if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
					t.Errorf("%q, handleGetBucket(). error unmarshaling json %v", tt.name, err)
				} else if !eq {
					t.Errorf("%q. handleGetBucket() = ***%s***", tt.name, diff)
				}
			}
		})
	}
}

func TestService_handlePostBucket(t *testing.T) {
	type fields struct {
		BucketService       platform.BucketService
		OrganizationService platform.OrganizationService
	}
	type args struct {
		bucket *platform.Bucket
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "create a new bucket",
			fields: fields{
				BucketService: &mock.BucketService{
					CreateBucketFn: func(ctx context.Context, c *platform.Bucket) error {
						c.ID = platformtesting.MustIDBase16("020f755c3c082000")
						return nil
					},
				},
				OrganizationService: &mock.OrganizationService{
					FindOrganizationF: func(ctx context.Context, f platform.OrganizationFilter) (*platform.Organization, error) {
						return &platform.Organization{ID: platformtesting.MustIDBase16("6f626f7274697320")}, nil
					},
				},
			},
			args: args{
				bucket: &platform.Bucket{
					Name:  "hello",
					OrgID: platformtesting.MustIDBase16("6f626f7274697320"),
				},
			},
			wants: wants{
				statusCode:  http.StatusCreated,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "org": "/api/v2/orgs/6f626f7274697320",
    "self": "/api/v2/buckets/020f755c3c082000",
    "logs": "/api/v2/buckets/020f755c3c082000/logs",
    "labels": "/api/v2/buckets/020f755c3c082000/labels",
    "members": "/api/v2/buckets/020f755c3c082000/members",
    "owners": "/api/v2/buckets/020f755c3c082000/owners",
    "write": "/api/v2/write?org=6f626f7274697320&bucket=020f755c3c082000"
  },
  "createdAt": "0001-01-01T00:00:00Z",
  "updatedAt": "0001-01-01T00:00:00Z",
  "id": "020f755c3c082000",
  "orgID": "6f626f7274697320",
	"type": "user",
  "name": "hello",
  "retentionRules": [],
  "labels": []
}
`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucketBackend := NewMockBucketBackend()
			bucketBackend.BucketService = tt.fields.BucketService
			bucketBackend.OrganizationService = tt.fields.OrganizationService
			h := NewBucketHandler(bucketBackend)

			b, err := json.Marshal(newBucket(tt.args.bucket))
			if err != nil {
				t.Fatalf("failed to unmarshal bucket: %v", err)
			}

			r := httptest.NewRequest("GET", "http://any.url?org=30", bytes.NewReader(b))
			w := httptest.NewRecorder()

			h.handlePostBucket(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handlePostBucket() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handlePostBucket() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if tt.wants.body != "" {
				if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
					t.Errorf("%q, handlePostBucket(). error unmarshaling json %v", tt.name, err)
				} else if !eq {
					t.Errorf("%q. handlePostBucket() = ***%s***", tt.name, diff)
				}
			}
		})
	}
}

func TestService_handleDeleteBucket(t *testing.T) {
	type fields struct {
		BucketService platform.BucketService
	}
	type args struct {
		id string
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "remove a bucket by id",
			fields: fields{
				&mock.BucketService{
					DeleteBucketFn: func(ctx context.Context, id platform.ID) error {
						if id == platformtesting.MustIDBase16("020f755c3c082000") {
							return nil
						}

						return fmt.Errorf("wrong id")
					},
				},
			},
			args: args{
				id: "020f755c3c082000",
			},
			wants: wants{
				statusCode: http.StatusNoContent,
			},
		},
		{
			name: "bucket not found",
			fields: fields{
				&mock.BucketService{
					DeleteBucketFn: func(ctx context.Context, id platform.ID) error {
						return &platform.Error{
							Code: platform.ENotFound,
							Msg:  "bucket not found",
						}
					},
				},
			},
			args: args{
				id: "020f755c3c082000",
			},
			wants: wants{
				statusCode: http.StatusNotFound,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucketBackend := NewMockBucketBackend()
			bucketBackend.HTTPErrorHandler = ErrorHandler(0)
			bucketBackend.BucketService = tt.fields.BucketService
			h := NewBucketHandler(bucketBackend)

			r := httptest.NewRequest("GET", "http://any.url", nil)

			r = r.WithContext(context.WithValue(
				context.Background(),
				httprouter.ParamsKey,
				httprouter.Params{
					{
						Key:   "id",
						Value: tt.args.id,
					},
				}))

			w := httptest.NewRecorder()

			h.handleDeleteBucket(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handleDeleteBucket() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handleDeleteBucket() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if tt.wants.body != "" {
				if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
					t.Errorf("%q, handleDeleteBucket(). error unmarshaling json %v", tt.name, err)
				} else if !eq {
					t.Errorf("%q. handleDeleteBucket() = ***%s***", tt.name, diff)
				}
			}
		})
	}
}

func TestService_handlePatchBucket(t *testing.T) {
	type fields struct {
		BucketService platform.BucketService
	}
	type args struct {
		id        string
		name      string
		retention time.Duration
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "update a bucket name and retention",
			fields: fields{
				&mock.BucketService{
					UpdateBucketFn: func(ctx context.Context, id platform.ID, upd platform.BucketUpdate) (*platform.Bucket, error) {
						if id == platformtesting.MustIDBase16("020f755c3c082000") {
							d := &platform.Bucket{
								ID:    platformtesting.MustIDBase16("020f755c3c082000"),
								Name:  "hello",
								OrgID: platformtesting.MustIDBase16("020f755c3c082000"),
							}

							if upd.Name != nil {
								d.Name = *upd.Name
							}

							if upd.RetentionPeriod != nil {
								d.RetentionPeriod = *upd.RetentionPeriod
							}

							return d, nil
						}

						return nil, fmt.Errorf("not found")
					},
				},
			},
			args: args{
				id:        "020f755c3c082000",
				name:      "example",
				retention: 2 * time.Second,
			},
			wants: wants{
				statusCode:  http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "org": "/api/v2/orgs/020f755c3c082000",
    "self": "/api/v2/buckets/020f755c3c082000",
    "logs": "/api/v2/buckets/020f755c3c082000/logs",
    "labels": "/api/v2/buckets/020f755c3c082000/labels",
    "members": "/api/v2/buckets/020f755c3c082000/members",
    "owners": "/api/v2/buckets/020f755c3c082000/owners",
    "write": "/api/v2/write?org=020f755c3c082000&bucket=020f755c3c082000"
  },
  "createdAt": "0001-01-01T00:00:00Z",
  "updatedAt": "0001-01-01T00:00:00Z",
  "id": "020f755c3c082000",
  "orgID": "020f755c3c082000",
	"type": "user",
  "name": "example",
  "retentionRules": [{"type": "expire", "everySeconds": 2}],
  "labels": []
}
`,
			},
		},
		{
			name: "bucket not found",
			fields: fields{
				&mock.BucketService{
					UpdateBucketFn: func(ctx context.Context, id platform.ID, upd platform.BucketUpdate) (*platform.Bucket, error) {
						return nil, &platform.Error{
							Code: platform.ENotFound,
							Msg:  "bucket not found",
						}
					},
				},
			},
			args: args{
				id:        "020f755c3c082000",
				name:      "hello",
				retention: time.Second,
			},
			wants: wants{
				statusCode: http.StatusNotFound,
			},
		},
		{
			name: "update bucket to no retention and new name",
			fields: fields{
				&mock.BucketService{
					UpdateBucketFn: func(ctx context.Context, id platform.ID, upd platform.BucketUpdate) (*platform.Bucket, error) {
						if id == platformtesting.MustIDBase16("020f755c3c082000") {
							d := &platform.Bucket{
								ID:    platformtesting.MustIDBase16("020f755c3c082000"),
								Name:  "hello",
								OrgID: platformtesting.MustIDBase16("020f755c3c082000"),
							}

							if upd.Name != nil {
								d.Name = *upd.Name
							}

							if upd.RetentionPeriod != nil {
								d.RetentionPeriod = *upd.RetentionPeriod
							}

							return d, nil
						}

						return nil, fmt.Errorf("not found")
					},
				},
			},
			args: args{
				id:        "020f755c3c082000",
				name:      "bucket with no retention",
				retention: 0,
			},
			wants: wants{
				statusCode:  http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "org": "/api/v2/orgs/020f755c3c082000",
    "self": "/api/v2/buckets/020f755c3c082000",
    "logs": "/api/v2/buckets/020f755c3c082000/logs",
    "labels": "/api/v2/buckets/020f755c3c082000/labels",
    "members": "/api/v2/buckets/020f755c3c082000/members",
    "owners": "/api/v2/buckets/020f755c3c082000/owners",
    "write": "/api/v2/write?org=020f755c3c082000&bucket=020f755c3c082000"
  },
  "createdAt": "0001-01-01T00:00:00Z",
  "updatedAt": "0001-01-01T00:00:00Z",
  "id": "020f755c3c082000",
  "orgID": "020f755c3c082000",
	"type": "user",
  "name": "bucket with no retention",
  "retentionRules": [],
  "labels": []
}
`,
			},
		},
		{
			name: "update retention policy to 'nothing'",
			fields: fields{
				&mock.BucketService{
					UpdateBucketFn: func(ctx context.Context, id platform.ID, upd platform.BucketUpdate) (*platform.Bucket, error) {
						if id == platformtesting.MustIDBase16("020f755c3c082000") {
							d := &platform.Bucket{
								ID:    platformtesting.MustIDBase16("020f755c3c082000"),
								Name:  "b1",
								OrgID: platformtesting.MustIDBase16("020f755c3c082000"),
							}

							if upd.Name != nil {
								d.Name = *upd.Name
							}

							if upd.RetentionPeriod != nil {
								d.RetentionPeriod = *upd.RetentionPeriod
							}

							return d, nil
						}

						return nil, &platform.Error{
							Code: platform.ENotFound,
							Msg:  "bucket not found",
						}
					},
				},
			},
			args: args{
				id:        "020f755c3c082000",
				retention: 0,
			},
			wants: wants{
				statusCode:  http.StatusOK,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "org": "/api/v2/orgs/020f755c3c082000",
    "self": "/api/v2/buckets/020f755c3c082000",
    "logs": "/api/v2/buckets/020f755c3c082000/logs",
    "labels": "/api/v2/buckets/020f755c3c082000/labels",
    "members": "/api/v2/buckets/020f755c3c082000/members",
    "owners": "/api/v2/buckets/020f755c3c082000/owners",
    "write": "/api/v2/write?org=020f755c3c082000&bucket=020f755c3c082000"
  },
  "createdAt": "0001-01-01T00:00:00Z",
  "updatedAt": "0001-01-01T00:00:00Z",
  "id": "020f755c3c082000",
  "orgID": "020f755c3c082000",
	"type": "user",
  "name": "b1",
  "retentionRules": [],
  "labels": []
}
`,
			},
		},
		{
			name: "update a bucket name with invalid retention policy is an error",
			fields: fields{
				&mock.BucketService{
					UpdateBucketFn: func(ctx context.Context, id platform.ID, upd platform.BucketUpdate) (*platform.Bucket, error) {
						if id == platformtesting.MustIDBase16("020f755c3c082000") {
							d := &platform.Bucket{
								ID:    platformtesting.MustIDBase16("020f755c3c082000"),
								Name:  "hello",
								OrgID: platformtesting.MustIDBase16("020f755c3c082000"),
							}

							if upd.Name != nil {
								d.Name = *upd.Name
							}

							if upd.RetentionPeriod != nil {
								d.RetentionPeriod = *upd.RetentionPeriod
							}

							return d, nil
						}

						return nil, &platform.Error{
							Code: platform.ENotFound,
							Msg:  "bucket not found",
						}
					},
				},
			},
			args: args{
				id:        "020f755c3c082000",
				name:      "example",
				retention: -10,
			},
			wants: wants{
				statusCode: http.StatusUnprocessableEntity,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucketBackend := NewMockBucketBackend()
			bucketBackend.HTTPErrorHandler = ErrorHandler(0)
			bucketBackend.BucketService = tt.fields.BucketService
			h := NewBucketHandler(bucketBackend)

			upd := platform.BucketUpdate{}
			if tt.args.name != "" {
				upd.Name = &tt.args.name
			}

			if tt.args.retention != 0 {
				upd.RetentionPeriod = &tt.args.retention
			}

			b, err := json.Marshal(newBucketUpdate(&upd))
			if err != nil {
				t.Fatalf("failed to unmarshal bucket update: %v", err)
			}

			r := httptest.NewRequest("GET", "http://any.url", bytes.NewReader(b))

			r = r.WithContext(context.WithValue(
				context.Background(),
				httprouter.ParamsKey,
				httprouter.Params{
					{
						Key:   "id",
						Value: tt.args.id,
					},
				}))

			w := httptest.NewRecorder()

			h.handlePatchBucket(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handlePatchBucket() = %v, want %v %v", tt.name, res.StatusCode, tt.wants.statusCode, w.Header())
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handlePatchBucket() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if tt.wants.body != "" {
				if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
					t.Errorf("%q, handlePatchBucket(). error unmarshaling json %v", tt.name, err)
				} else if !eq {
					t.Errorf("%q. handlePatchBucket() = ***%s***", tt.name, diff)
				}
			}
		})
	}
}

func TestService_handlePostBucketMember(t *testing.T) {
	type fields struct {
		UserService platform.UserService
	}
	type args struct {
		bucketID string
		user     *platform.User
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "add a bucket member",
			fields: fields{
				UserService: &mock.UserService{
					FindUserByIDFn: func(ctx context.Context, id platform.ID) (*platform.User, error) {
						return &platform.User{
							ID:     id,
							Name:   "name",
							Status: platform.Active,
						}, nil
					},
				},
			},
			args: args{
				bucketID: "020f755c3c082000",
				user: &platform.User{
					ID: platformtesting.MustIDBase16("6f626f7274697320"),
				},
			},
			wants: wants{
				statusCode:  http.StatusCreated,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "logs": "/api/v2/users/6f626f7274697320/logs",
    "self": "/api/v2/users/6f626f7274697320"
  },
  "role": "member",
  "id": "6f626f7274697320",
	"name": "name",
	"status": "active"
}
`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucketBackend := NewMockBucketBackend()
			bucketBackend.UserService = tt.fields.UserService
			h := NewBucketHandler(bucketBackend)

			b, err := json.Marshal(tt.args.user)
			if err != nil {
				t.Fatalf("failed to marshal user: %v", err)
			}

			path := fmt.Sprintf("/api/v2/buckets/%s/members", tt.args.bucketID)
			r := httptest.NewRequest("POST", path, bytes.NewReader(b))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handlePostBucketMember() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handlePostBucketMember() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
				t.Errorf("%q, handlePostBucketMember(). error unmarshaling json %v", tt.name, err)
			} else if tt.wants.body != "" && !eq {
				t.Errorf("%q. handlePostBucketMember() = ***%s***", tt.name, diff)
			}
		})
	}
}

func TestService_handlePostBucketOwner(t *testing.T) {
	type fields struct {
		UserService platform.UserService
	}
	type args struct {
		bucketID string
		user     *platform.User
	}
	type wants struct {
		statusCode  int
		contentType string
		body        string
	}

	tests := []struct {
		name   string
		fields fields
		args   args
		wants  wants
	}{
		{
			name: "add a bucket owner",
			fields: fields{
				UserService: &mock.UserService{
					FindUserByIDFn: func(ctx context.Context, id platform.ID) (*platform.User, error) {
						return &platform.User{
							ID:     id,
							Name:   "name",
							Status: platform.Active,
						}, nil
					},
				},
			},
			args: args{
				bucketID: "020f755c3c082000",
				user: &platform.User{
					ID: platformtesting.MustIDBase16("6f626f7274697320"),
				},
			},
			wants: wants{
				statusCode:  http.StatusCreated,
				contentType: "application/json; charset=utf-8",
				body: `
{
  "links": {
    "logs": "/api/v2/users/6f626f7274697320/logs",
    "self": "/api/v2/users/6f626f7274697320"
  },
  "role": "owner",
  "id": "6f626f7274697320",
	"name": "name",
	"status": "active"
}
`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucketBackend := NewMockBucketBackend()
			bucketBackend.UserService = tt.fields.UserService
			h := NewBucketHandler(bucketBackend)

			b, err := json.Marshal(tt.args.user)
			if err != nil {
				t.Fatalf("failed to marshal user: %v", err)
			}

			path := fmt.Sprintf("/api/v2/buckets/%s/owners", tt.args.bucketID)
			r := httptest.NewRequest("POST", path, bytes.NewReader(b))
			w := httptest.NewRecorder()

			h.ServeHTTP(w, r)

			res := w.Result()
			content := res.Header.Get("Content-Type")
			body, _ := ioutil.ReadAll(res.Body)

			if res.StatusCode != tt.wants.statusCode {
				t.Errorf("%q. handlePostBucketOwner() = %v, want %v", tt.name, res.StatusCode, tt.wants.statusCode)
			}
			if tt.wants.contentType != "" && content != tt.wants.contentType {
				t.Errorf("%q. handlePostBucketOwner() = %v, want %v", tt.name, content, tt.wants.contentType)
			}
			if eq, diff, err := jsonEqual(string(body), tt.wants.body); err != nil {
				t.Errorf("%q, handlePostBucketOwner(). error unmarshaling json %v", tt.name, err)
			} else if tt.wants.body != "" && !eq {
				t.Errorf("%q. handlePostBucketOwner() = ***%s***", tt.name, diff)
			}
		})
	}
}

func initBucketService(f platformtesting.BucketFields, t *testing.T) (platform.BucketService, string, func()) {
	svc := kv.NewService(inmem.NewKVStore())
	svc.IDGenerator = f.IDGenerator
	svc.OrgBucketIDs = f.OrgBucketIDs
	svc.TimeGenerator = f.TimeGenerator
	if f.TimeGenerator == nil {
		svc.TimeGenerator = platform.RealTimeGenerator{}
	}

	ctx := context.Background()
	if err := svc.Initialize(ctx); err != nil {
		t.Fatal(err)
	}

	for _, o := range f.Organizations {
		if err := svc.PutOrganization(ctx, o); err != nil {
			t.Fatalf("failed to populate organizations")
		}
	}
	for _, b := range f.Buckets {
		if err := svc.PutBucket(ctx, b); err != nil {
			t.Fatalf("failed to populate buckets")
		}
	}

	bucketBackend := NewMockBucketBackend()
	bucketBackend.HTTPErrorHandler = ErrorHandler(0)
	bucketBackend.BucketService = svc
	bucketBackend.OrganizationService = svc
	handler := NewBucketHandler(bucketBackend)
	server := httptest.NewServer(handler)
	client := BucketService{
		Addr:     server.URL,
		OpPrefix: inmem.OpPrefix,
	}
	done := server.Close

	return &client, inmem.OpPrefix, done
}

func TestBucketService(t *testing.T) {
	platformtesting.BucketService(initBucketService, t)
}
