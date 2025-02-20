package pkger

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/influxdata/influxdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Run("pkg has all necessary metadata", func(t *testing.T) {
		t.Run("has valid metadata and at least 1 resource", func(t *testing.T) {
			testfileRunner(t, "testdata/bucket", nil)
		})

		t.Run("malformed required metadata", func(t *testing.T) {
			tests := []testPkgResourceError{
				{
					name: "missing apiVersion",
					pkgStr: `kind: Package
meta:
  pkgName:      first_bucket_package
  pkgVersion:   1
spec:
  resources:
    - kind: Bucket
      name: buck_1
      retention_period: 1h
`,
					valFields: []string{"apiVersion"},
				},
				{
					name: "apiVersion is invalid version",
					pkgStr: `apiVersion: 222.2 #invalid apiVersion
kind: Package
meta:
  pkgName:      first_bucket_package
  pkgVersion:   1
spec:
  resources:
    - kind: Bucket
      name: buck_1
      retention_period: 1h
`,
					valFields: []string{"apiVersion"},
				},
				{
					name: "missing kind",
					pkgStr: `apiVersion: 0.1.0
meta:
  pkgName:   first_bucket_package
  pkgVersion:   1
spec:
  resources:
    - kind: Bucket
      name: buck_1
      retention_period: 1h
`,
					valFields: []string{"kind"},
				},
				{
					name: "missing pkgName",
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgVersion:   1
spec:
  resources:
    - kind: Bucket
      name: buck_1
      retention_period: 1h
`,
					valFields: []string{"meta.pkgName"},
				},
				{
					name: "missing pkgVersion",
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:   foo_name
spec:
  resources:
    - kind: Bucket
      name: buck_1
      retention_period: 1h
`,
					valFields: []string{"meta.pkgVersion"},
				},
				{
					name: "missing multiple",
					pkgStr: `spec:
  resources:
    - kind: Bucket
      name: buck_1
      retention_period: 1h
`,
					valFields: []string{"apiVersion", "kind", "meta.pkgVersion", "meta.pkgName"},
				},
			}

			for _, tt := range tests {
				testPkgErrors(t, KindPackage, tt)
			}
		})
	})

	t.Run("pkg with a bucket", func(t *testing.T) {
		t.Run("with valid bucket pkg should be valid", func(t *testing.T) {
			testfileRunner(t, "testdata/bucket", func(t *testing.T, pkg *Pkg) {
				buckets := pkg.buckets()
				require.Len(t, buckets, 1)

				actual := buckets[0]
				expectedBucket := bucket{
					Name:            "rucket_11",
					Description:     "bucket 1 description",
					RetentionPeriod: time.Hour,
				}
				assert.Equal(t, expectedBucket, *actual)
			})
		})

		t.Run("handles bad config", func(t *testing.T) {
			tests := []testPkgResourceError{
				{
					name:           "missing name",
					validationErrs: 1,
					valFields:      []string{"name"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      first_bucket_package
  pkgVersion:   1
spec:
  resources:
    - kind: Bucket
      retention_period: 1h
`,
				},
				{
					name:           "mixed valid and missing name",
					validationErrs: 1,
					valFields:      []string{"name"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      first_bucket_package
  pkgVersion:   1
spec:
  resources:
    - kind: Bucket
      retention_period: 1h
      name: valid name
    - kind: Bucket
      retention_period: 1h
`,
				},
				{
					name:           "mixed valid and multiple bad names",
					resourceErrs:   2,
					validationErrs: 1,
					valFields:      []string{"name"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      first_bucket_package
  pkgVersion:   1
spec:
  resources:
    - kind: Bucket
      retention_period: 1h
      name: valid name
    - kind: Bucket
      retention_period: 1h
    - kind: Bucket
      retention_period: 1h
`,
				},
				{
					name:           "duplicate bucket names",
					resourceErrs:   1,
					validationErrs: 1,
					valFields:      []string{"name"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      first_bucket_package
  pkgVersion:   1
spec:
  resources:
    - kind: Bucket
      retention_period: 1h
      name: valid name
    - kind: Bucket
      retention_period: 1h
      name: valid name
`,
				},
			}

			for _, tt := range tests {
				testPkgErrors(t, KindBucket, tt)
			}
		})
	})

	t.Run("pkg with a label", func(t *testing.T) {
		t.Run("with valid label pkg should be valid", func(t *testing.T) {
			testfileRunner(t, "testdata/label", func(t *testing.T, pkg *Pkg) {
				labels := pkg.labels()
				require.Len(t, labels, 2)

				expectedLabel1 := label{
					Name:        "label_1",
					Description: "label 1 description",
					Color:       "#FFFFFF",
				}
				assert.Equal(t, expectedLabel1, *labels[0])

				expectedLabel2 := label{
					Name:        "label_2",
					Description: "label 2 description",
					Color:       "#000000",
				}
				assert.Equal(t, expectedLabel2, *labels[1])
			})
		})

		t.Run("with missing label name should error", func(t *testing.T) {
			tests := []testPkgResourceError{
				{
					name:           "missing name",
					validationErrs: 1,
					valFields:      []string{"name"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName: first_label_pkg 
  pkgVersion:   1
spec:
  resources:
    - kind: Label 
`,
				},
				{
					name:           "mixed valid and missing name",
					validationErrs: 1,
					valFields:      []string{"name"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName: label_pkg
  pkgVersion: 1
spec:
  resources:
    - kind: Label
      name: valid name
    - kind: Label
`,
				},
				{
					name:           "multiple labels with missing name",
					resourceErrs:   2,
					validationErrs: 1,
					valFields:      []string{"name"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName: label_pkg
  pkgVersion: 1
spec:
  resources:
    - kind: Label
    - kind: Label
`,
				},
			}

			for _, tt := range tests {
				testPkgErrors(t, KindLabel, tt)
			}
		})
	})

	t.Run("pkg with buckets and labels associated", func(t *testing.T) {
		testfileRunner(t, "testdata/bucket_associates_label", func(t *testing.T, pkg *Pkg) {
			sum := pkg.Summary()
			require.Len(t, sum.Labels, 2)

			bkts := sum.Buckets
			require.Len(t, bkts, 3)

			expectedLabels := []struct {
				bktName string
				labels  []string
			}{
				{
					bktName: "rucket_1",
					labels:  []string{"label_1"},
				},
				{
					bktName: "rucket_2",
					labels:  []string{"label_2"},
				},
				{
					bktName: "rucket_3",
					labels:  []string{"label_1", "label_2"},
				},
			}
			for i, expected := range expectedLabels {
				bkt := bkts[i]
				require.Len(t, bkt.LabelAssociations, len(expected.labels))

				for j, label := range expected.labels {
					assert.Equal(t, label, bkt.LabelAssociations[j].Name)
				}
			}

			expectedMappings := []SummaryLabelMapping{
				{
					ResourceName: "rucket_1",
					LabelName:    "label_1",
				},
				{
					ResourceName: "rucket_2",
					LabelName:    "label_2",
				},
				{
					ResourceName: "rucket_3",
					LabelName:    "label_1",
				},
				{
					ResourceName: "rucket_3",
					LabelName:    "label_2",
				},
			}

			require.Len(t, sum.LabelMappings, len(expectedMappings))
			for i, expected := range expectedMappings {
				expected.LabelMapping.ResourceType = influxdb.BucketsResourceType
				assert.Equal(t, expected, sum.LabelMappings[i])
			}
		})

		t.Run("association doesn't exist then provides an error", func(t *testing.T) {
			tests := []testPkgResourceError{
				{
					name:    "no labels provided",
					assErrs: 1,
					assIdxs: []int{0},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName: label_pkg
  pkgVersion: 1
spec:
  resources:
    - kind: Bucket
      name: buck_1
      associations:
        - kind: Label
          name: label_1
`,
				},
				{
					name:    "mixed found and not found",
					assErrs: 1,
					assIdxs: []int{1},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName: label_pkg
  pkgVersion: 1
spec:
  resources:
    - kind: Label
      name: label_1
    - kind: Bucket
      name: buck_1
      associations:
        - kind: Label
          name: label_1
        - kind: Label
          name: unfound label
`,
				},
				{
					name:    "multiple not found",
					assErrs: 1,
					assIdxs: []int{0, 1},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName: label_pkg
  pkgVersion: 1
spec:
  resources:
    - kind: Label
      name: label_1
    - kind: Bucket
      name: buck_1
      associations:
        - kind: Label
          name: not found 1
        - kind: Label
          name: unfound label
`,
				},
				{
					name:    "duplicate valid nested labels",
					assErrs: 1,
					assIdxs: []int{1},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName: label_pkg
  pkgVersion: 1
spec:
  resources:
    - kind: Label
      name: label_1
    - kind: Bucket
      name: buck_1
      associations:
        - kind: Label
          name: label_1
        - kind: Label
          name: label_1
`,
				},
			}

			for _, tt := range tests {
				testPkgErrors(t, KindBucket, tt)
			}
		})
	})

	t.Run("pkg with single dashboard and single chart", func(t *testing.T) {
		t.Run("single stat chart", func(t *testing.T) {
			testfileRunner(t, "testdata/dashboard", func(t *testing.T, pkg *Pkg) {
				sum := pkg.Summary()
				require.Len(t, sum.Dashboards, 1)

				actual := sum.Dashboards[0]
				assert.Equal(t, "dash_1", actual.Name)
				assert.Equal(t, "desc1", actual.Description)

				require.Len(t, actual.Charts, 1)
				actualChart := actual.Charts[0]
				assert.Equal(t, 3, actualChart.Height)
				assert.Equal(t, 6, actualChart.Width)
				assert.Equal(t, 1, actualChart.XPosition)
				assert.Equal(t, 2, actualChart.YPosition)

				props, ok := actualChart.Properties.(influxdb.SingleStatViewProperties)
				require.True(t, ok)
				assert.Equal(t, "single-stat", props.GetType())
				assert.Equal(t, "single stat note", props.Note)
				assert.True(t, props.ShowNoteWhenEmpty)
				assert.True(t, props.DecimalPlaces.IsEnforced)
				assert.Equal(t, int32(1), props.DecimalPlaces.Digits)
				assert.Equal(t, "days", props.Suffix)
				assert.Equal(t, "sumtin", props.Prefix)

				require.Len(t, props.Queries, 1)
				q := props.Queries[0]
				queryText := `from(bucket: v.bucket) |> range(start: v.timeRangeStart) |> filter(fn: (r) => r._measurement == "processes") |> filter(fn: (r) => r._field == "running" or r._field == "blocked") |> aggregateWindow(every: v.windowPeriod, fn: max) |> yield(name: "max")`
				assert.Equal(t, queryText, q.Text)
				assert.Equal(t, "advanced", q.EditMode)

				require.Len(t, props.ViewColors, 1)
				c := props.ViewColors[0]
				assert.NotZero(t, c.ID)
				assert.Equal(t, "laser", c.Name)
				assert.Equal(t, "text", c.Type)
				assert.Equal(t, "#8F8AF4", c.Hex)
				assert.Equal(t, 3.0, c.Value)
			})

			t.Run("handles invalid config", func(t *testing.T) {
				tests := []testPkgResourceError{
					{
						name:           "color missing hex value",
						validationErrs: 1,
						valFields:      []string{"charts[0].colors[0].hex"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat
          name:   single stat
          suffix: days
          width:  6
          height: 3
          shade: true
          queries:
            - query: >
                from(bucket: v.bucket) |> range(start: v.timeRangeStart) |> filter(fn: (r) => r._measurement == "system") |> filter(fn: (r) => r._field == "uptime") |> last() |> map(fn: (r) => ({r with _value: r._value / 86400})) |> yield(name: "last")
          colors:
            - name: laser
              type: text
`,
					},
					{
						name:           "query missing text value",
						validationErrs: 1,
						valFields:      []string{"charts[0].queries[0].query"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat
          name:   single stat
          suffix: days
          width:  6
          height: 3
          shade: true
          queries:
            - query: 
          colors:
            - name: laser
              type: text
              hex: "#aaa222"
`,
					},
					{
						name:           "no queries provided",
						validationErrs: 1,
						valFields:      []string{"charts[0].queries"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat
          name:   single stat
          suffix: days
          width:  6
          height: 3
          shade: true
          colors:
            - name: laser
              type: text
              hex: "#aaa222"
`,
					},
					{
						name:           "no width provided",
						validationErrs: 1,
						valFields:      []string{"charts[0].width"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat
          name:   single stat
          suffix: days
          height: 3
          shade: true
          queries:
            - query: >
                from(bucket: v.bucket) |> range(start: v.timeRangeStart) |> filter(fn: (r) => r._measurement == "system") |> filter(fn: (r) => r._field == "uptime") |> last() |> map(fn: (r) => ({r with _value: r._value / 86400})) |> yield(name: "last")
          colors:
            - name: laser
              type: text
              hex: "#aaa333"
`,
					},
					{
						name:           "no height provided",
						validationErrs: 1,
						valFields:      []string{"charts[0].height"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat
          name:   single stat
          suffix: days
          width: 3
          shade: true
          queries:
            - query: >
                from(bucket: v.bucket) |> range(start: v.timeRangeStart) |> filter(fn: (r) => r._measurement == "system") |> filter(fn: (r) => r._field == "uptime") |> last() |> map(fn: (r) => ({r with _value: r._value / 86400})) |> yield(name: "last")
          colors:
            - name: laser
              type: text
              hex: "#aaa333"
`,
					},
				}

				for _, tt := range tests {
					testPkgErrors(t, KindDashboard, tt)
				}
			})
		})

		t.Run("single stat plus line chart", func(t *testing.T) {
			testfileRunner(t, "testdata/dashboard_single_stat_plus_line", func(t *testing.T, pkg *Pkg) {
				sum := pkg.Summary()
				require.Len(t, sum.Dashboards, 1)

				actual := sum.Dashboards[0]
				assert.Equal(t, "dash_1", actual.Name)
				assert.Equal(t, "desc1", actual.Description)

				require.Len(t, actual.Charts, 1)
				actualChart := actual.Charts[0]
				assert.Equal(t, 3, actualChart.Height)
				assert.Equal(t, 6, actualChart.Width)
				assert.Equal(t, 1, actualChart.XPosition)
				assert.Equal(t, 2, actualChart.YPosition)

				props, ok := actualChart.Properties.(influxdb.LinePlusSingleStatProperties)
				require.True(t, ok)
				assert.Equal(t, "single stat plus line note", props.Note)
				assert.True(t, props.ShowNoteWhenEmpty)
				assert.True(t, props.DecimalPlaces.IsEnforced)
				assert.Equal(t, int32(1), props.DecimalPlaces.Digits)
				assert.Equal(t, "days", props.Suffix)
				assert.Equal(t, "sumtin", props.Prefix)
				assert.Equal(t, "leg_type", props.Legend.Type)
				assert.Equal(t, "horizontal", props.Legend.Orientation)

				require.Len(t, props.Queries, 1)
				q := props.Queries[0]
				expectedQuery := `from(bucket: v.bucket)  |> range(start: v.timeRangeStart)  |> filter(fn: (r) => r._measurement == "mem")  |> filter(fn: (r) => r._field == "used_percent")  |> aggregateWindow(every: v.windowPeriod, fn: mean, createEmpty: false)  |> yield(name: "mean")`
				assert.Equal(t, expectedQuery, q.Text)
				assert.Equal(t, "advanced", q.EditMode)

				for _, key := range []string{"x", "y"} {
					xAxis, ok := props.Axes[key]
					require.True(t, ok, "key="+key)
					assert.Equal(t, "10", xAxis.Base, "key="+key)
					assert.Equal(t, key+"_label", xAxis.Label, "key="+key)
					assert.Equal(t, key+"_prefix", xAxis.Prefix, "key="+key)
					assert.Equal(t, "linear", xAxis.Scale, "key="+key)
					assert.Equal(t, key+"_suffix", xAxis.Suffix, "key="+key)
				}

				require.Len(t, props.ViewColors, 2)
				c := props.ViewColors[0]
				assert.NotZero(t, c.ID)
				assert.Equal(t, "laser", c.Name)
				assert.Equal(t, "text", c.Type)
				assert.Equal(t, "#8F8AF4", c.Hex)
				assert.Equal(t, 3.0, c.Value)

				c = props.ViewColors[1]
				assert.NotZero(t, c.ID)
				assert.Equal(t, "android", c.Name)
				assert.Equal(t, "scale", c.Type)
				assert.Equal(t, "#F4CF31", c.Hex)
				assert.Equal(t, 1.0, c.Value)
			})

			t.Run("handles invalid config", func(t *testing.T) {
				tests := []testPkgResourceError{
					{
						name:           "color missing hex value",
						validationErrs: 1,
						valFields:      []string{"charts[0].colors[0].hex"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat_Plus_Line
          name:   single stat plus line
          width:  6
          height: 3
          queries:
            - query: >
                from(bucket: v.bucket) |> range(start: v.timeRangeStart) |> filter(fn: (r) => r._measurement == "system") |> filter(fn: (r) => r._field == "uptime") |> last() |> map(fn: (r) => ({r with _value: r._value / 86400})) |> yield(name: "last")
          colors:
            - name: laser
              type: text
            - name: android
              type: scale
              hex: "#F4CF31"
          axes:
            - name : "x"
              label: x_label
              base: 10
              scale: linear
            - name: "y"
              label: y_label
              base: 10
              scale: linear
`,
					},
					{
						name:           "missing query value",
						validationErrs: 1,
						valFields:      []string{"charts[0].queries[0].query"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat_Plus_Line
          name:   single stat plus line
          width:  6
          height: 3
          queries:
            - query:  
          colors:
            - name: laser
              type: text
              hex: "#abcabc"
            - name: android
              type: scale
              hex: "#F4CF31"
          axes:
            - name : "x"
              label: x_label
              base: 10
              scale: linear
            - name: "y"
              label: y_label
              base: 10
              scale: linear
`,
					},
					{
						name:           "no queries provided",
						validationErrs: 1,
						valFields:      []string{"charts[0].queries"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat_Plus_Line
          name:   single stat plus line
          width:  6
          height: 3
          colors:
            - name: laser
              type: text
              hex: "red"
            - name: android
              type: scale
              hex: "#F4CF31"
          axes:
            - name : "x"
              label: x_label
              base: 10
              scale: linear
            - name: "y"
              label: y_label
              base: 10
              scale: linear`,
					},
					{
						name:           "no width provided",
						validationErrs: 1,
						valFields:      []string{"charts[0].width"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat_Plus_Line
          name:   single stat plus line
          height: 3
          queries:
            - query: >
                from(bucket: v.bucket) |> range(start: v.timeRangeStart) |> filter(fn: (r) => r._measurement == "system") |> filter(fn: (r) => r._field == "uptime") |> last() |> map(fn: (r) => ({r with _value: r._value / 86400})) |> yield(name: "last")
          colors:
            - name: laser
              type: text
              hex: green
            - name: android
              type: scale
              hex: "#F4CF31"
          axes:
            - name : "x"
              label: x_label
              base: 10
              scale: linear
            - name: "y"
              label: y_label
              base: 10
              scale: linear`,
					},
					{
						name:           "no height provided",
						validationErrs: 1,
						valFields:      []string{"charts[0].height"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat_Plus_Line
          name:   single stat plus line
          width: 3
          queries:
            - query: >
                from(bucket: v.bucket) |> range(start: v.timeRangeStart) |> filter(fn: (r) => r._measurement == "system") |> filter(fn: (r) => r._field == "uptime") |> last() |> map(fn: (r) => ({r with _value: r._value / 86400})) |> yield(name: "last")
          colors:
            - name: laser
              type: text
              hex: green
            - name: android
              type: scale
              hex: "#F4CF31"
          axes:
            - name : "x"
              label: x_label
              base: 10
              scale: linear
            - name: "y"
              label: y_label
              base: 10
              scale: linear`,
					},
					{
						name:           "missing text color but has scale color",
						validationErrs: 1,
						valFields:      []string{"charts[0].colors"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat_Plus_Line
          name:   single stat plus line
          width: 3
          height: 3
          queries:
            - query: >
                from(bucket: v.bucket) |> range(start: v.timeRangeStart) |> filter(fn: (r) => r._measurement == "system") |> filter(fn: (r) => r._field == "uptime") |> last() |> map(fn: (r) => ({r with _value: r._value / 86400})) |> yield(name: "last")
          colors:
            - name: android
              type: scale
              hex: "#F4CF31"
          axes:
            - name : "x"
              label: x_label
              base: 10
              scale: linear
            - name: "y"
              label: y_label
              base: 10
              scale: linear`,
					},
					{
						name:           "missing x axis",
						validationErrs: 1,
						valFields:      []string{"charts[0].axes"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat_Plus_Line
          name:   single stat plus line
          width: 3
          height: 3
          queries:
            - query: >
                from(bucket: v.bucket) |> range(start: v.timeRangeStart) |> filter(fn: (r) => r._measurement == "system") |> filter(fn: (r) => r._field == "uptime") |> last() |> map(fn: (r) => ({r with _value: r._value / 86400})) |> yield(name: "last")
          colors:
            - name: first
              type: text
              hex: "#aabbaa"
            - name: android
              type: scale
              hex: "#F4CF31"
          axes:
            - name: "y"
              label: y_label
              base: 10
              scale: linear`,
					},
					{
						name:           "missing y axis",
						validationErrs: 1,
						valFields:      []string{"charts[0].axes"},
						pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      description: desc1
      charts:
        - kind:   Single_Stat_Plus_Line
          name:   single stat plus line
          width: 3
          height: 3
          queries:
            - query: >
                from(bucket: v.bucket) |> range(start: v.timeRangeStart) |> filter(fn: (r) => r._measurement == "system") |> filter(fn: (r) => r._field == "uptime") |> last() |> map(fn: (r) => ({r with _value: r._value / 86400})) |> yield(name: "last")
          colors:
            - name: first
              type: text
              hex: "#aabbaa"
            - name: android
              type: scale
              hex: "#F4CF31"
          axes:
            - name: "x"
              base: 10
              scale: linear`,
					},
				}

				for _, tt := range tests {
					testPkgErrors(t, KindDashboard, tt)
				}
			})
		})

		t.Run("pkg with single dashboard xy chart", func(t *testing.T) {
			t.Run("xy chart", func(t *testing.T) {
				testfileRunner(t, "testdata/dashboard_xy", func(t *testing.T, pkg *Pkg) {
					sum := pkg.Summary()
					require.Len(t, sum.Dashboards, 1)

					actual := sum.Dashboards[0]
					assert.Equal(t, "dash_1", actual.Name)
					assert.Equal(t, "desc1", actual.Description)

					require.Len(t, actual.Charts, 1)
					actualChart := actual.Charts[0]
					assert.Equal(t, 3, actualChart.Height)
					assert.Equal(t, 6, actualChart.Width)
					assert.Equal(t, 1, actualChart.XPosition)
					assert.Equal(t, 2, actualChart.YPosition)

					props, ok := actualChart.Properties.(influxdb.XYViewProperties)
					require.True(t, ok)
					assert.Equal(t, "xy", props.GetType())
					assert.Equal(t, true, props.ShadeBelow)
					assert.Equal(t, "xy chart note", props.Note)
					assert.True(t, props.ShowNoteWhenEmpty)

					require.Len(t, props.Queries, 1)
					q := props.Queries[0]
					queryText := `from(bucket: v.bucket)  |> range(start: v.timeRangeStart, stop: v.timeRangeStop)  |> filter(fn: (r) => r._measurement == "boltdb_writes_total")  |> filter(fn: (r) => r._field == "counter")`
					assert.Equal(t, queryText, q.Text)
					assert.Equal(t, "advanced", q.EditMode)

					require.Len(t, props.ViewColors, 1)
					c := props.ViewColors[0]
					assert.NotZero(t, c.ID)
					assert.Equal(t, "laser", c.Name)
					assert.Equal(t, "scale", c.Type)
					assert.Equal(t, "#8F8AF4", c.Hex)
					assert.Equal(t, 3.0, c.Value)
				})
			})

			t.Run("handles invalid config", func(t *testing.T) {
				tests := []testPkgResourceError{
					{
						name:           "color missing hex value",
						encoding:       EncodingJSON,
						validationErrs: 1,
						valFields:      []string{"charts[0].colors[0].hex"},
						pkgStr: `{
						"apiVersion": "0.1.0",
						"kind": "Package",
						"meta": {
						  "pkgName": "pkg_name",
						  "pkgVersion": "1",
						  "description": "pack description"
						},
						"spec": {
						  "resources": [
							{
							  "kind": "Dashboard",
							  "name": "dash_1",
							  "description": "desc1",
							  "charts": [
								{
								  "kind": "XY",
								  "name": "xy chart",
								  "prefix": "sumtin",
								  "note": "xy chart note",
								  "noteOnEmpty": true,
								  "xPos": 1,
								  "yPos": 2,
								  "width": 6,
								  "height": 3,
								  "decimalPlaces": 1,
								  "shade": true,
								  "xColumn": "_time",
								  "yColumn": "_value",
								  "legend": {},
								  "queries": [
									{
									  "query": "from(bucket: v.bucket)  |> range(start: v.timeRangeStart, stop: v.timeRangeStop)  |> filter(fn: (r) => r._measurement == \"boltdb_writes_total\")  |> filter(fn: (r) => r._field == \"counter\")"
									}
								  ],
								  "colors": [
									{
									  "name": "laser",
									  "type": "scale",
									  "value": 3
									}
								  ],
								  "axes":[
									{
									  "name": "x",
									  "label": "x_label",
									  "prefix": "x_prefix",
									  "suffix": "x_suffix",
									  "base": "10",
									  "scale": "linear"
									},
									{
									  "name": "y",
									  "label": "y_label",
									  "prefix": "y_prefix",
									  "suffix": "y_suffix",
									  "base": "10",
									  "scale": "linear"
									}
								  ],
								  "geom": "line"
								}
							  ]
							}
						  ]
						}
					  }		  
`,
					},
					{
						name:           "invalid geom flag",
						encoding:       EncodingJSON,
						validationErrs: 1,
						valFields:      []string{"charts[0].geom"},
						pkgStr: `
					{
						"apiVersion": "0.1.0",
						"kind": "Package",
						"meta": {
						  "pkgName": "pkg_name",
						  "pkgVersion": "1",
						  "description": "pack description"
						},
						"spec": {
						  "resources": [
							{
							  "kind": "Dashboard",
							  "name": "dash_1",
							  "description": "desc1",
							  "charts": [
								{
								  "kind": "XY",
								  "name": "xy chart",
								  "prefix": "sumtin",
								  "note": "xy chart note",
								  "noteOnEmpty": true,
								  "xPos": 1,
								  "yPos": 2,
								  "width": 6,
								  "height": 3,
								  "decimalPlaces": 1,
								  "shade": true,
								  "xColumn": "_time",
								  "yColumn": "_value",
								  "legend": {},
								  "queries": [
									{
									  "query": "from(bucket: v.bucket)  |> range(start: v.timeRangeStart, stop: v.timeRangeStop)  |> filter(fn: (r) => r._measurement == \"boltdb_writes_total\")  |> filter(fn: (r) => r._field == \"counter\")"
									}
								  ],
								  "colors": [
									{
									  "name": "laser",
									  "type": "scale",
									  "value": 3,
									  "hex": "#FFF000"
									}
								  ],
								  "axes":[
									{
									  "name": "x",
									  "label": "x_label",
									  "prefix": "x_prefix",
									  "suffix": "x_suffix",
									  "base": "10",
									  "scale": "linear"
									},
									{
									  "name": "y",
									  "label": "y_label",
									  "prefix": "y_prefix",
									  "suffix": "y_suffix",
									  "base": "10",
									  "scale": "linear"
									}
								  ],
								  "geom": "huzzah"
								}
							  ]
							}
						  ]
						}
					  }  
`,
					},
				}

				for _, tt := range tests {
					testPkgErrors(t, KindDashboard, tt)
				}
			})
		})

		t.Run("pkg with single dashboard gauge chart", func(t *testing.T) {
			testfileRunner(t, "testdata/dashboard_gauge", func(t *testing.T, pkg *Pkg) {
				sum := pkg.Summary()
				require.Len(t, sum.Dashboards, 1)

				actual := sum.Dashboards[0]
				assert.Equal(t, "dash_1", actual.Name)
				assert.Equal(t, "desc1", actual.Description)

				require.Len(t, actual.Charts, 1)
				actualChart := actual.Charts[0]
				assert.Equal(t, 3, actualChart.Height)
				assert.Equal(t, 6, actualChart.Width)
				assert.Equal(t, 1, actualChart.XPosition)
				assert.Equal(t, 2, actualChart.YPosition)

				props, ok := actualChart.Properties.(influxdb.GaugeViewProperties)
				require.True(t, ok)
				assert.Equal(t, "gauge", props.GetType())
				assert.Equal(t, "gauge note", props.Note)
				assert.True(t, props.ShowNoteWhenEmpty)

				require.Len(t, props.Queries, 1)
				q := props.Queries[0]
				queryText := `from(bucket: v.bucket)  |> range(start: v.timeRangeStart, stop: v.timeRangeStop)  |> filter(fn: (r) => r._measurement == "boltdb_writes_total")  |> filter(fn: (r) => r._field == "counter")`
				assert.Equal(t, queryText, q.Text)
				assert.Equal(t, "advanced", q.EditMode)

				require.Len(t, props.ViewColors, 3)
				c := props.ViewColors[0]
				assert.NotZero(t, c.ID)
				assert.Equal(t, "laser", c.Name)
				assert.Equal(t, "min", c.Type)
				assert.Equal(t, "#8F8AF4", c.Hex)
				assert.Equal(t, 0.0, c.Value)
			})

			t.Run("handles invalid config", func(t *testing.T) {
				tests := []testPkgResourceError{
					{
						name:           "color a gauge type",
						encoding:       EncodingJSON,
						validationErrs: 1,
						valFields:      []string{"charts[0].colors"},
						pkgStr: `
{
	"apiVersion": "0.1.0",
	"kind": "Package",
	"meta": {
		"pkgName": "pkg_name",
		"pkgVersion": "1",
		"description": "pack description"
	},
	"spec": {
		"resources": [
		{
			"kind": "Dashboard",
			"name": "dash_1",
			"description": "desc1",
			"charts": [
			{
				"kind": "gauge",
				"name": "gauge",
				"prefix": "prefix",
				"suffix": "suffix",
				"note": "gauge note",
				"noteOnEmpty": true,
				"xPos": 1,
				"yPos": 2,
				"width": 6,
				"height": 3,
				"decimalPlaces": 1,
				"xColumn": "_time",
				"yColumn": "_value",
				"queries": [
				{
					"query": "from(bucket: v.bucket)  |> range(start: v.timeRangeStart, stop: v.timeRangeStop)  |> filter(fn: (r) => r._measurement == \"boltdb_writes_total\")  |> filter(fn: (r) => r._field == \"counter\")"
				}
				],
				"colors": [
				{
					"name": "laser",
					"type": "min",
					"hex": "#8F8AF4",
					"value": 0
				},
				{
					"name": "comet",
					"type": "max",
					"hex": "#F4CF31",
					"value": 5000
					}
				]
			}
			]
		}
		]
	}
}  
`,
					},
					{
						name:           "color mixing a hex value",
						encoding:       EncodingJSON,
						validationErrs: 1,
						valFields:      []string{"charts[0].colors[0].hex"},
						pkgStr: `
{
	"apiVersion": "0.1.0",
	"kind": "Package",
	"meta": {
		"pkgName": "pkg_name",
		"pkgVersion": "1",
		"description": "pack description"
	},
	"spec": {
		"resources": [
		{
			"kind": "Dashboard",
			"name": "dash_1",
			"description": "desc1",
			"charts": [
			{
				"kind": "gauge",
				"name": "gauge",
				"prefix": "prefix",
				"suffix": "suffix",
				"note": "gauge note",
				"noteOnEmpty": true,
				"xPos": 1,
				"yPos": 2,
				"width": 6,
				"height": 3,
				"decimalPlaces": 1,
				"xColumn": "_time",
				"yColumn": "_value",
				"queries": [
				{
					"query": "from(bucket: v.bucket)  |> range(start: v.timeRangeStart, stop: v.timeRangeStop)  |> filter(fn: (r) => r._measurement == \"boltdb_writes_total\")  |> filter(fn: (r) => r._field == \"counter\")"
				}
				],
				"colors": [
				{
					"name": "laser",
					"type": "min",
					"value": 0
				},
				{
					"name": "pool",
					"type": "threshold",
					"hex": "#F4CF31",
					"value": 700
				},
				{
					"name": "comet",
					"type": "max",
					"hex": "#F4CF31",
					"value": 5000
					}
				]
			}
			]
		}
		]
	}
}
`,
					},
					{
						name:           "missing a query value",
						encoding:       EncodingJSON,
						validationErrs: 1,
						valFields:      []string{"charts[0].queries[0].query"},
						pkgStr: `
{
	"apiVersion": "0.1.0",
	"kind": "Package",
	"meta": {
		"pkgName": "pkg_name",
		"pkgVersion": "1",
		"description": "pack description"
	},
	"spec": {
		"resources": [
		{
			"kind": "Dashboard",
			"name": "dash_1",
			"description": "desc1",
			"charts": [
			{
				"kind": "gauge",
				"name": "gauge",
				"prefix": "prefix",
				"suffix": "suffix",
				"note": "gauge note",
				"noteOnEmpty": true,
				"xPos": 1,
				"yPos": 2,
				"width": 6,
				"height": 3,
				"decimalPlaces": 1,
				"xColumn": "_time",
				"yColumn": "_value",
				"queries": [
					{
						"query": null
					}
				],
				"colors": [
				{
					"name": "laser",
					"type": "min",
					"hex": "#FFF000",
					"value": 0
				},
				{
					"name": "pool",
					"type": "threshold",
					"hex": "#F4CF31",
					"value": 700
				},
				{
					"name": "comet",
					"type": "max",
					"hex": "#F4CF31",
					"value": 5000
					}
				]
			}
			]
		}
		]
	}
}
`,
					},
				}

				for _, tt := range tests {
					testPkgErrors(t, KindDashboard, tt)
				}
			})
		})
	})

	t.Run("pkg with dashboard and labels associated", func(t *testing.T) {
		testfileRunner(t, "testdata/dashboard_associates_label", func(t *testing.T, pkg *Pkg) {
			sum := pkg.Summary()
			require.Len(t, sum.Dashboards, 1)

			actual := sum.Dashboards[0]
			assert.Equal(t, "dash_1", actual.Name)
			assert.Equal(t, "desc1", actual.Description)

			require.Len(t, actual.LabelAssociations, 1)
			actualLabel := actual.LabelAssociations[0]
			assert.Equal(t, "label_1", actualLabel.Name)

			expectedMappings := []SummaryLabelMapping{
				{
					ResourceName: "dash_1",
					LabelName:    "label_1",
				},
			}
			require.Len(t, sum.LabelMappings, len(expectedMappings))

			for i, expected := range expectedMappings {
				expected.LabelMapping.ResourceType = influxdb.DashboardsResourceType
				assert.Equal(t, expected, sum.LabelMappings[i])
			}
		})

		t.Run("association doesn't exist then provides an error", func(t *testing.T) {
			tests := []testPkgResourceError{
				{
					name:    "no labels provided",
					assErrs: 1,
					assIdxs: []int{0},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName: label_pkg
  pkgVersion: 1
spec:
  resources:
    - kind: Dashboard
      name: dash_1
      associations:
        - kind: Label
          name: label_1
`,
				},
				{
					name:    "mixed found and not found",
					assErrs: 1,
					assIdxs: []int{1},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName: label_pkg
  pkgVersion: 1
spec:
  resources:
    - kind: Label
      name: label_1
    - kind: Dashboard
      name: dash_1
      associations:
        - kind: Label
          name: label_1
        - kind: Label
          name: unfound label
`,
				},
				{
					name:    "multiple not found",
					assErrs: 1,
					assIdxs: []int{0, 1},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName: label_pkg
  pkgVersion: 1
spec:
  resources:
    - kind: Label
      name: label_1
    - kind: Dashboard
      name: dash_1
      associations:
        - kind: Label
          name: not found 1
        - kind: Label
          name: unfound label
`,
				},
				{
					name:    "duplicate valid nested labels",
					assErrs: 1,
					assIdxs: []int{1},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName: label_pkg
  pkgVersion: 1
spec:
  resources:
    - kind: Label
      name: label_1
    - kind: Dashboard
      name: dash_1
      associations:
        - kind: Label
          name: label_1
        - kind: Label
          name: label_1
`,
				},
			}

			for _, tt := range tests {
				testPkgErrors(t, KindDashboard, tt)
			}
		})
	})

	t.Run("pkg with a variable", func(t *testing.T) {
		t.Run("with valid fields should produce summary", func(t *testing.T) {
			testfileRunner(t, "testdata/variables", func(t *testing.T, pkg *Pkg) {
				sum := pkg.Summary()

				require.Len(t, sum.Variables, 4)

				varEquals := func(t *testing.T, name, vType string, vals interface{}, v SummaryVariable) {
					t.Helper()

					assert.Equal(t, name, v.Name)
					assert.Equal(t, name+" desc", v.Description)
					require.NotNil(t, v.Arguments)
					assert.Equal(t, vType, v.Arguments.Type)
					assert.Equal(t, vals, v.Arguments.Values)
				}

				// validates we support all known variable types
				varEquals(t,
					"var_const",
					"constant",
					influxdb.VariableConstantValues([]string{"first val"}),
					sum.Variables[0],
				)

				varEquals(t,
					"var_map",
					"map",
					influxdb.VariableMapValues{"k1": "v1"},
					sum.Variables[1],
				)

				varEquals(t,
					"var_query_1",
					"query",
					influxdb.VariableQueryValues{
						Query:    `buckets()  |> filter(fn: (r) => r.name !~ /^_/)  |> rename(columns: {name: "_value"})  |> keep(columns: ["_value"])`,
						Language: "flux",
					},
					sum.Variables[2],
				)

				varEquals(t,
					"var_query_2",
					"query",
					influxdb.VariableQueryValues{
						Query:    "an influxql query of sorts",
						Language: "influxql",
					},
					sum.Variables[3],
				)
			})
		})

		t.Run("handles bad config", func(t *testing.T) {
			tests := []testPkgResourceError{
				{
					name:           "name missing",
					validationErrs: 1,
					valFields:      []string{"name"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Variable
      type: map
      values:
        k1: v1
`,
				},
				{
					name:           "map var missing values",
					validationErrs: 1,
					valFields:      []string{"values"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Variable
      name: var
      type: map
`,
				},
				{
					name:           "const var missing values",
					validationErrs: 1,
					valFields:      []string{"values"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Variable
      name: var
      type: constant
`,
				},
				{
					name:           "query var missing query",
					validationErrs: 1,
					valFields:      []string{"query"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Variable
      name: var
      type: query
      language: influxql
`,
				},
				{
					name:           "query var missing query language",
					validationErrs: 1,
					valFields:      []string{"language"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Variable
      name: var
      type: query
      query: from(v.bucket) |> count()
`,
				},
				{
					name:           "query var provides incorrect query language",
					validationErrs: 1,
					valFields:      []string{"language"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Variable
      name: var
      type: query
      query: from(v.bucket) |> count()
      language: wrongo language
`,
				},
				{
					name:           "duplicate var names",
					validationErrs: 1,
					valFields:      []string{"name"},
					pkgStr: `apiVersion: 0.1.0
kind: Package
meta:
  pkgName:      pkg_name
  pkgVersion:   1
  description:  pack description
spec:
  resources:
    - kind: Variable
      name: var
      type: query
      query: from(v.bucket) |> count()
      language: flux
    - kind: Variable
      name: var
      type: query
      query: from(v.bucket) |> mean()
      language: flux
`,
				},
			}

			for _, tt := range tests {
				testPkgErrors(t, KindVariable, tt)
			}
		})
	})

	t.Run("pkg with variable and labels associated", func(t *testing.T) {
		testfileRunner(t, "testdata/variables_associates_label.yml", func(t *testing.T, pkg *Pkg) {
			sum := pkg.Summary()
			require.Len(t, sum.Labels, 1)

			vars := sum.Variables
			require.Len(t, vars, 1)

			expectedLabelMappings := []struct {
				varName string
				labels  []string
			}{
				{
					varName: "var_1",
					labels:  []string{"label_1"},
				},
			}
			for i, expected := range expectedLabelMappings {
				v := vars[i]
				require.Len(t, v.LabelAssociations, len(expected.labels))

				for j, label := range expected.labels {
					assert.Equal(t, label, v.LabelAssociations[j].Name)
				}
			}

			expectedMappings := []SummaryLabelMapping{
				{
					ResourceName: "var_1",
					LabelName:    "label_1",
				},
			}

			require.Len(t, sum.LabelMappings, len(expectedMappings))
			for i, expected := range expectedMappings {
				expected.LabelMapping.ResourceType = influxdb.VariablesResourceType
				assert.Equal(t, expected, sum.LabelMappings[i])
			}
		})
	})
}

type testPkgResourceError struct {
	name           string
	encoding       Encoding
	pkgStr         string
	resourceErrs   int
	validationErrs int
	valFields      []string
	assErrs        int
	assIdxs        []int
}

// defaults to yaml encoding if encoding not provided
// defaults num resources to 1 if resource errs not provided.
func testPkgErrors(t *testing.T, k Kind, tt testPkgResourceError) {
	t.Helper()
	encoding := EncodingYAML
	if tt.encoding != EncodingUnknown {
		encoding = tt.encoding
	}

	resErrs := 1
	if tt.resourceErrs > 0 {
		resErrs = tt.resourceErrs
	}

	fn := func(t *testing.T) {
		t.Helper()

		_, err := Parse(encoding, FromString(tt.pkgStr))
		require.Error(t, err)

		pErr, ok := IsParseErr(err)
		require.True(t, ok, err)

		require.Len(t, pErr.Resources, resErrs)

		resErr := pErr.Resources[0]
		assert.Equal(t, k.String(), resErr.Kind)

		require.Len(t, resErr.ValidationFails, len(tt.valFields))
		for i, vFail := range resErr.ValidationFails {
			assert.Equal(t, tt.valFields[i], vFail.Field)
		}

		assFails := pErr.Resources[0].AssociationFails
		require.Len(t, assFails, len(tt.assIdxs))
		if tt.assErrs == 0 {
			return
		}

		for i, f := range assFails {
			assert.Equal(t, "associations", assFails[i].Field)
			assert.Equal(t, tt.assIdxs[i], f.Index)
		}
	}
	t.Run(tt.name, fn)
}

type baseAsserts struct {
	version     string
	kind        string
	description string
	metaName    string
	metaVersion string
}

func validParsedPkg(t *testing.T, path string, encoding Encoding, expected baseAsserts) *Pkg {
	t.Helper()

	pkg, err := Parse(encoding, FromFile(path))
	require.NoError(t, err)

	require.Equal(t, expected.version, pkg.APIVersion)
	require.Equal(t, expected.kind, pkg.Kind)
	require.Equal(t, expected.description, pkg.Metadata.Description)
	require.Equal(t, expected.metaName, pkg.Metadata.Name)
	require.Equal(t, expected.metaVersion, pkg.Metadata.Version)
	require.True(t, pkg.isParsed)

	return pkg
}

func testfileRunner(t *testing.T, path string, testFn func(t *testing.T, pkg *Pkg)) {
	t.Helper()

	tests := []struct {
		name      string
		extension string
		encoding  Encoding
	}{
		{
			name:      "yaml",
			extension: ".yml",
			encoding:  EncodingYAML,
		},
		{
			name:      "json",
			extension: ".json",
			encoding:  EncodingJSON,
		},
	}

	ext := filepath.Ext(path)
	switch ext {
	case ".yml":
		tests = tests[:1]
	case ".json":
		tests = tests[1:]
	}

	path = strings.TrimSuffix(path, ext)

	for _, tt := range tests {
		fn := func(t *testing.T) {
			t.Helper()

			pkg := validParsedPkg(t, path+tt.extension, tt.encoding, baseAsserts{
				version:     "0.1.0",
				kind:        "Package",
				description: "pack description",
				metaName:    "pkg_name",
				metaVersion: "1",
			})
			if testFn != nil {
				testFn(t, pkg)
			}
		}
		t.Run(tt.name, fn)
	}
}
