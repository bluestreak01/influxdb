package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/influxdata/influxdb"
	"github.com/influxdata/influxdb/http"
	"github.com/influxdata/influxdb/pkger"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	input "github.com/tcnksm/go-input"
)

func pkgCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pkg",
		Short: "Create a reusable pkg to create resources in a declarative manner",
	}

	path := cmd.Flags().String("path", "", "path to manifest file")
	cmd.MarkFlagFilename("path", "yaml", "yml", "json")
	cmd.MarkFlagRequired("path")

	orgID := cmd.Flags().String("org-id", "", "The ID of the organization that owns the bucket")
	cmd.MarkFlagRequired("org-id")

	hasColor := cmd.Flags().Bool("color", true, "Enable color in output, defaults true")
	hasTableBorders := cmd.Flags().Bool("table-borders", true, "Enable table borders, defaults true")

	cmd.RunE = pkgApply(orgID, path, hasColor, hasTableBorders)

	return cmd
}

func pkgApply(orgID, path *string, hasColor, hasTableBorders *bool) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) (e error) {
		if !*hasColor {
			color.NoColor = true
		}

		influxOrgID, err := influxdb.IDFromString(*orgID)
		if err != nil {
			return err
		}

		svc, err := newPkgerSVC(flags)
		if err != nil {
			return err
		}

		pkg, err := pkgFromFile(*path)
		if err != nil {
			return err
		}

		_, diff, err := svc.DryRun(context.Background(), *influxOrgID, pkg)
		if err != nil {
			return err
		}

		printPkgDiff(*hasColor, *hasTableBorders, diff)

		ui := &input.UI{
			Writer: os.Stdout,
			Reader: os.Stdin,
		}

		confirm := getInput(ui, "Confirm application of the above resources (y/n)", "n")
		if strings.ToLower(confirm) != "y" {
			fmt.Fprintln(os.Stdout, "aborted application of package")
			return nil
		}

		summary, err := svc.Apply(context.Background(), *influxOrgID, pkg)
		if err != nil {
			return err
		}

		printPkgSummary(*hasColor, *hasTableBorders, summary)

		return nil
	}
}

func newPkgerSVC(f Flags) (*pkger.Service, error) {
	bucketSVC, err := newBucketService(f)
	if err != nil {
		return nil, err
	}

	labelSVC, err := newLabelService(f)
	if err != nil {
		return nil, err
	}

	dashSVC, err := newDashboardService(f)
	if err != nil {
		return nil, err
	}

	varSVC, err := newVariableService(f)
	if err != nil {
		return nil, err
	}

	return pkger.NewService(
		pkger.WithBucketSVC(bucketSVC),
		pkger.WithDashboardSVC(dashSVC),
		pkger.WithLabelSVC(labelSVC),
		pkger.WithVariableSVC(varSVC),
	), nil
}

func newDashboardService(f Flags) (influxdb.DashboardService, error) {
	if f.local {
		return newLocalKVService()
	}
	return &http.DashboardService{
		Addr:  f.host,
		Token: f.token,
	}, nil
}

func newLabelService(f Flags) (influxdb.LabelService, error) {
	if f.local {
		return newLocalKVService()
	}
	return &http.LabelService{
		Addr:  f.host,
		Token: f.token,
	}, nil
}

func newVariableService(f Flags) (influxdb.VariableService, error) {
	if f.local {
		return newLocalKVService()
	}
	return &http.VariableService{
		Addr:  f.host,
		Token: f.token,
	}, nil
}

func pkgFromFile(path string) (*pkger.Pkg, error) {
	var enc pkger.Encoding
	switch ext := filepath.Ext(path); ext {
	case ".yaml", ".yml":
		enc = pkger.EncodingYAML
	case ".json":
		enc = pkger.EncodingJSON
	default:
		return nil, errors.New("file provided must be one of yaml/yml/json extension but got: " + ext)
	}

	return pkger.Parse(enc, pkger.FromFile(path))
}

func printPkgDiff(hasColor, hasTableBorders bool, diff pkger.Diff) {
	red := color.New(color.FgRed).SprintfFunc()
	green := color.New(color.FgHiGreen, color.Bold).SprintfFunc()

	strDiff := func(isNew bool, old, new string) string {
		if isNew {
			return green(new)
		}
		if old == new {
			return new
		}
		return fmt.Sprintf("%s\n%s", red("%q", old), green("%q", new))
	}

	boolDiff := func(b bool) string {
		bb := strconv.FormatBool(b)
		if b {
			return green(bb)
		}
		return bb
	}

	durDiff := func(isNew bool, oldDur, newDur time.Duration) string {
		o := oldDur.String()
		if oldDur == 0 {
			o = "inf"
		}
		n := newDur.String()
		if newDur == 0 {
			n = "inf"
		}
		if isNew {
			return green(n)
		}
		if oldDur == newDur {
			return n
		}
		return fmt.Sprintf("%s\n%s", red(o), green(n))
	}

	tablePrintFn := tablePrinterGen(hasColor, hasTableBorders)
	if labels := diff.Labels; len(labels) > 0 {
		headers := []string{"New", "ID", "Name", "Color", "Description"}
		tablePrintFn("LABELS", headers, len(labels), func(w *tablewriter.Table) {
			for _, l := range labels {
				w.Append([]string{
					boolDiff(l.IsNew()),
					l.ID.String(),
					l.Name,
					strDiff(l.IsNew(), l.OldColor, l.NewColor),
					strDiff(l.IsNew(), l.OldDesc, l.NewDesc),
				})
			}
		})
	}

	if bkts := diff.Buckets; len(bkts) > 0 {
		headers := []string{"New", "ID", "Name", "Retention Period", "Description"}
		tablePrintFn("BUCKETS", headers, len(bkts), func(w *tablewriter.Table) {
			for _, b := range bkts {
				w.Append([]string{
					boolDiff(b.IsNew()),
					b.ID.String(),
					b.Name,
					durDiff(b.IsNew(), b.OldRetention, b.NewRetention),
					strDiff(b.IsNew(), b.OldDesc, b.NewDesc),
				})
			}
		})
	}

	if dashes := diff.Dashboards; len(dashes) > 0 {
		headers := []string{"New", "Name", "Description", "Num Charts"}
		tablePrintFn("DASHBOARDS", headers, len(dashes), func(w *tablewriter.Table) {
			for _, d := range dashes {
				w.Append([]string{
					boolDiff(true),
					d.Name,
					green(d.Desc),
					green(strconv.Itoa(len(d.Charts))),
				})
			}
		})
	}

	if vars := diff.Variables; len(vars) > 0 {
		headers := []string{"New", "ID", "Name", "Description", "Arg Type", "Arg Values"}
		tablePrintFn("VARIABLES", headers, len(vars), func(w *tablewriter.Table) {
			for _, v := range vars {
				var oldArgType string
				if v.OldArgs != nil {
					oldArgType = v.OldArgs.Type
				}
				var newArgType string
				if v.NewArgs != nil {
					newArgType = v.NewArgs.Type
				}
				w.Append([]string{
					boolDiff(v.IsNew()),
					v.ID.String(),
					v.Name,
					strDiff(v.IsNew(), v.OldDesc, v.NewDesc),
					strDiff(v.IsNew(), oldArgType, newArgType),
					strDiff(v.IsNew(), printVarArgs(v.OldArgs), printVarArgs(v.NewArgs)),
				})
			}
		})
	}

	if len(diff.LabelMappings) > 0 {
		headers := []string{"New", "Resource Type", "Resource Name", "Resource ID", "Label Name", "Label ID"}
		tablePrintFn("LABEL MAPPINGS", headers, len(diff.LabelMappings), func(w *tablewriter.Table) {
			for _, m := range diff.LabelMappings {
				w.Append([]string{
					boolDiff(m.IsNew),
					string(m.ResType),
					m.ResName,
					m.ResID.String(),
					m.LabelName,
					m.LabelID.String(),
				})
			}
		})
	}
}

func printVarArgs(a *influxdb.VariableArguments) string {
	if a == nil {
		return "<nil>"
	}
	if a.Type == "map" {
		b, err := json.Marshal(a.Values)
		if err != nil {
			return "{}"
		}
		return string(b)
	}
	if a.Type == "constant" {
		vals, ok := a.Values.(influxdb.VariableConstantValues)
		if !ok {
			return "[]"
		}
		var out []string
		for _, s := range vals {
			out = append(out, fmt.Sprintf("%q", s))
		}
		return fmt.Sprintf("[%s]", strings.Join(out, " "))
	}
	if a.Type == "query" {
		qVal, ok := a.Values.(influxdb.VariableQueryValues)
		if !ok {
			return ""
		}
		return fmt.Sprintf("language=%q query=%q", qVal.Language, qVal.Query)
	}
	return "unknown variable argument"
}

func printPkgSummary(hasColor, hasTableBorders bool, sum pkger.Summary) {
	tablePrintFn := tablePrinterGen(hasColor, hasTableBorders)
	if labels := sum.Labels; len(labels) > 0 {
		headers := []string{"ID", "Name", "Description", "Color"}
		tablePrintFn("LABELS", headers, len(labels), func(w *tablewriter.Table) {
			for _, l := range labels {
				w.Append([]string{
					l.ID.String(),
					l.Name,
					l.Properties["description"],
					l.Properties["color"],
				})
			}
		})
	}

	if buckets := sum.Buckets; len(buckets) > 0 {
		headers := []string{"ID", "Name", "Retention", "Description"}
		tablePrintFn("BUCKETS", headers, len(buckets), func(w *tablewriter.Table) {
			for _, bucket := range buckets {
				w.Append([]string{
					bucket.ID.String(),
					bucket.Name,
					formatDuration(bucket.RetentionPeriod),
					bucket.Description,
				})
			}
		})
	}

	if dashes := sum.Dashboards; len(dashes) > 0 {
		headers := []string{"ID", "Name", "Description"}
		tablePrintFn("DASHBOARDS", headers, len(dashes), func(w *tablewriter.Table) {
			for _, d := range dashes {
				w.Append([]string{
					d.ID.String(),
					d.Name,
					d.Description,
				})
			}
		})
	}

	if vars := sum.Variables; len(vars) > 0 {
		headers := []string{"ID", "Name", "Description", "Arg Type", "Arg Values"}
		tablePrintFn("VARIABLES", headers, len(vars), func(w *tablewriter.Table) {
			for _, v := range vars {
				args := v.Arguments
				w.Append([]string{
					v.ID.String(),
					v.Name,
					v.Description,
					args.Type,
					printVarArgs(args),
				})
			}
		})
	}

	if mappings := sum.LabelMappings; len(mappings) > 0 {
		headers := []string{"Resource Type", "Resource Name", "Resource ID", "Label Name", "Label ID"}
		tablePrintFn("LABEL MAPPINGS", headers, len(mappings), func(w *tablewriter.Table) {
			for _, m := range mappings {
				w.Append([]string{
					string(m.ResourceType),
					m.ResourceName,
					m.ResourceID.String(),
					m.LabelName,
					m.LabelID.String(),
				})
			}
		})
	}
}

func tablePrinterGen(hasColor, hasTableBorder bool) func(table string, headers []string, count int, appendFn func(w *tablewriter.Table)) {
	return func(table string, headers []string, count int, appendFn func(w *tablewriter.Table)) {
		tablePrinter(table, headers, count, hasColor, hasTableBorder, appendFn)
	}
}

func tablePrinter(table string, headers []string, count int, hasColor, hasTableBorders bool, appendFn func(w *tablewriter.Table)) {
	descrCol := -1
	for i, h := range headers {
		if strings.ToLower(h) == "description" {
			descrCol = i
			break
		}
	}

	w := tablewriter.NewWriter(os.Stdout)
	w.SetBorder(hasTableBorders)
	w.SetRowLine(hasTableBorders)

	var alignments []int
	for range headers {
		alignments = append(alignments, tablewriter.ALIGN_CENTER)
	}
	if descrCol != -1 {
		w.SetColMinWidth(descrCol, 30)
		alignments[descrCol] = tablewriter.ALIGN_LEFT
	}

	color.New(color.FgYellow, color.Bold).Fprintln(os.Stdout, strings.ToUpper(table))
	w.SetHeader(headers)
	w.SetColumnAlignment(alignments)

	appendFn(w)

	footers := make([]string, len(headers))
	footers[len(footers)-2] = "TOTAL"
	footers[len(footers)-1] = strconv.Itoa(count)
	w.SetFooter(footers)
	if hasColor {
		var colors []tablewriter.Colors
		for i := 0; i < len(headers); i++ {
			colors = append(colors, tablewriter.Color(tablewriter.FgHiCyanColor))
		}
		w.SetHeaderColor(colors...)
		colors[len(colors)-2] = tablewriter.Color(tablewriter.FgHiBlueColor)
		colors[len(colors)-1] = tablewriter.Color(tablewriter.FgHiBlueColor)
		w.SetFooterColor(colors...)
	}

	w.Render()
	fmt.Fprintln(os.Stdout)
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "inf"
	}
	return d.String()
}
