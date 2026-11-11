package services

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphReportCreateKeepsPortOrderAndDisabledState(t *testing.T) {
	f := newGraphReportFixture(t)
	in := reportInput(f.group.ID, "Uplink", "Backup")
	in.Enabled = false

	report, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)

	assert.False(t, report.Enabled)
	assert.Equal(t, "TEST MRTG", report.GroupName)
	assert.Nil(t, report.LastSentOn)
	require.Len(t, report.Ports, 2)
	assert.Equal(t, "Uplink", report.Ports[0].Label)
	assert.Equal(t, 0, report.Ports[0].Position)
	assert.Equal(t, "Backup", report.Ports[1].Label)
	assert.Equal(t, 1, report.Ports[1].Position)
}

func TestGraphReportOneGroupOneReport(t *testing.T) {
	f := newGraphReportFixture(t)
	f.createReport(t, "Uplink")

	_, err := f.reports.Create(reportInput(f.group.ID, "Lain"), uuid.New())
	assert.True(t, errors.Is(err, ErrGraphReportDuplicate))
}

func TestGraphReportUpdateMayKeepItsOwnGroupAndReplacesPorts(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink", "Backup")

	updated, err := f.reports.Update(report.ID, reportInput(f.group.ID, "Baru"))
	require.NoError(t, err)
	require.Len(t, updated.Ports, 1)
	assert.Equal(t, "Baru", updated.Ports[0].Label)
}

func TestGraphReportMessageIsStoredAndRewritten(t *testing.T) {
	f := newGraphReportFixture(t)
	in := reportInput(f.group.ID, "Uplink")
	in.Message = "  Halo {pelanggan}  "
	report, err := f.reports.Create(in, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "Halo {pelanggan}", report.Message)

	in.Message = ""
	updated, err := f.reports.Update(report.ID, in)
	require.NoError(t, err)
	assert.Empty(t, updated.Message, "blank goes back to the default greeting")

	in.Message = strings.Repeat("é", MaxGraphReportMessageRunes)
	_, err = f.reports.Update(report.ID, in)
	assert.NoError(t, err, "the limit counts characters, not bytes")
}

func TestGraphReportCaptionFallsBackToTheDefault(t *testing.T) {
	assert.Equal(t, strings.ReplaceAll(DefaultGraphReportMessage, "{pelanggan}", "PT A"), GraphReportCaption(" \n", "PT A"))
	assert.Equal(t, "Untuk PT A", GraphReportCaption("Untuk {pelanggan}", "PT A"))
}

func TestGraphReportPortCountBounds(t *testing.T) {
	f := newGraphReportFixture(t)
	for _, count := range []int{0, MaxGraphReportPorts + 1} {
		labels := make([]string, count)
		for i := range labels {
			labels[i] = fmt.Sprintf("P%d", i)
		}
		_, err := f.reports.Create(reportInput(f.group.ID, labels...), uuid.New())
		assert.True(t, errors.Is(err, ErrValidation), "%d ports", count)
	}

	labels := make([]string, MaxGraphReportPorts)
	for i := range labels {
		labels[i] = fmt.Sprintf("P%d", i)
	}
	_, err := f.reports.Create(reportInput(f.group.ID, labels...), uuid.New())
	assert.NoError(t, err)
}

func TestGraphReportRefusesBlankFieldsAndUnknownGroup(t *testing.T) {
	f := newGraphReportFixture(t)
	cases := map[string]func(*GraphReportInput){
		"customer":      func(in *GraphReportInput) { in.CustomerName = "  " },
		"if_name":       func(in *GraphReportInput) { in.Ports[0].IfName = "" },
		"label":         func(in *GraphReportInput) { in.Ports[0].Label = " " },
		"unknown group": func(in *GraphReportInput) { in.WAGroupID = uuid.New() },
		"long message":  func(in *GraphReportInput) { in.Message = strings.Repeat("a", MaxGraphReportMessageRunes+1) },
	}
	for name, mutate := range cases {
		in := reportInput(f.group.ID, "Uplink")
		mutate(&in)
		_, err := f.reports.Create(in, uuid.New())
		assert.True(t, errors.Is(err, ErrValidation), name)
	}
}

func TestGraphReportDeleteTakesThePorts(t *testing.T) {
	f := newGraphReportFixture(t)
	report := f.createReport(t, "Uplink")

	require.NoError(t, f.reports.Delete(report.ID))

	reports, err := f.reports.List()
	require.NoError(t, err)
	assert.Empty(t, reports)
	var ports int64
	require.NoError(t, f.db.Table("graph_report_ports").Count(&ports).Error)
	assert.Zero(t, ports)
}
