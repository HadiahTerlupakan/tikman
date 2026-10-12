package netflow

import (
	"bytes"
	"errors"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/netflow/ipfixtest"
)

const flowTemplate = 256

func decodeAll(t *testing.T, tmpl *Templates, messages ...[]byte) Result {
	t.Helper()
	var total Result
	for _, m := range messages {
		res, err := tmpl.Decode(m)
		require.NoError(t, err)
		total.Flows = append(total.Flows, res.Flows...)
		total.MissingTemplate += res.MissingTemplate
		total.RejectedTemplates += res.RejectedTemplates
	}
	return total
}

func oneFlow() []byte {
	return ipfixtest.FlowRecord("198.51.100.7", "203.0.113.5", 10, 20, 1500, 3)
}

func TestDecodeReadsFlowsAfterTheirTemplate(t *testing.T) {
	msg := ipfixtest.Message(1,
		ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...),
		ipfixtest.DataSet(flowTemplate, oneFlow(),
			ipfixtest.FlowRecord("203.0.113.6", "192.0.2.80", 20, 11, 400, 2)))

	res := decodeAll(t, NewTemplates(), msg)

	require.Len(t, res.Flows, 2)
	assert.Equal(t, Flow{Src: netip.MustParseAddr("198.51.100.7"), Dst: netip.MustParseAddr("203.0.113.5"),
		Ingress: 10, Egress: 20, Bytes: 1500, Packets: 3}, res.Flows[0])
	assert.Equal(t, uint32(11), res.Flows[1].Egress)
}

// Review Focus 1: after flowd or the router restarts, data arrives before the
// template describing it. It is dropped and counted, never misread, and read
// normally once the template comes.
func TestDecodeDropsDataUntilItsTemplateArrives(t *testing.T) {
	tmpl := NewTemplates()
	data := ipfixtest.Message(1, ipfixtest.DataSet(flowTemplate, oneFlow()))

	early := decodeAll(t, tmpl, data)
	assert.Empty(t, early.Flows)
	assert.Equal(t, 1, early.MissingTemplate)

	late := decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...)), data)
	assert.Len(t, late.Flows, 1)
	assert.Zero(t, late.MissingTemplate)
}

// Review Focus 1: a rebooted router re-sends a template ID with its fields in
// another order and other widths; the new layout replaces the old one.
func TestDecodeReplacesATemplateResentWithAnotherLayout(t *testing.T) {
	tmpl := NewTemplates()
	decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...)))

	swapped := []ipfixtest.Field{{ID: 12, Length: 4}, {ID: 8, Length: 4}, {ID: 1, Length: 4},
		{ID: 10, Length: 2}, {ID: 14, Length: 2}}
	record := bytes.Join([][]byte{ipfixtest.IPv4("203.0.113.5"), ipfixtest.IPv4("198.51.100.7"),
		ipfixtest.U32(900), ipfixtest.U16(10), ipfixtest.U16(20)}, nil)
	res := decodeAll(t, tmpl, ipfixtest.Message(1,
		ipfixtest.TemplateSet(flowTemplate, swapped...), ipfixtest.DataSet(flowTemplate, record)))

	require.Len(t, res.Flows, 1)
	assert.Equal(t, netip.MustParseAddr("198.51.100.7"), res.Flows[0].Src)
	assert.Equal(t, netip.MustParseAddr("203.0.113.5"), res.Flows[0].Dst)
	assert.Equal(t, uint64(900), res.Flows[0].Bytes, "a four-byte counter is reduced-size encoding")
	assert.Equal(t, uint32(10), res.Flows[0].Ingress)
}

func TestDecodeKeepsTemplatesPerObservationDomain(t *testing.T) {
	tmpl := NewTemplates()
	decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...)))

	res := decodeAll(t, tmpl, ipfixtest.Message(2, ipfixtest.DataSet(flowTemplate, oneFlow())))

	assert.Empty(t, res.Flows)
	assert.Equal(t, 1, res.MissingTemplate)
}

// Review Focus 5: a firmware update adds an enterprise field numbered like
// sourceIPv4Address and a variable-length string, once short and once in the
// three-byte length form. Neither may shift or overwrite the fields read.
func TestDecodeSkipsEnterpriseAndVariableLengthFields(t *testing.T) {
	fields := append([]ipfixtest.Field{
		{ID: 8, Length: 4, Enterprise: 32473},
		{ID: 82, Length: ipfixtest.VariableLength},
	}, ipfixtest.FlowFields...)
	short := bytes.Join([][]byte{ipfixtest.IPv4("192.0.2.200"), {3}, []byte("eth"), oneFlow()}, nil)
	long := bytes.Join([][]byte{ipfixtest.IPv4("192.0.2.201"), {255}, ipfixtest.U16(300),
		bytes.Repeat([]byte("x"), 300), ipfixtest.FlowRecord("198.51.100.8", "203.0.113.6", 10, 20, 700, 1)}, nil)

	res := decodeAll(t, NewTemplates(), ipfixtest.Message(1,
		ipfixtest.TemplateSet(flowTemplate, fields...), ipfixtest.DataSet(flowTemplate, short, long)))

	require.Len(t, res.Flows, 2)
	assert.Equal(t, netip.MustParseAddr("198.51.100.7"), res.Flows[0].Src, "the enterprise field is not sourceIPv4Address")
	assert.Equal(t, netip.MustParseAddr("198.51.100.8"), res.Flows[1].Src)
	assert.Equal(t, uint64(700), res.Flows[1].Bytes)
}

func TestDecodeRefusesWhatIsNotWholeIPFIX(t *testing.T) {
	valid := ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...))
	v9 := bytes.Clone(valid)
	v9[1] = 9
	longer := bytes.Clone(valid)
	longer[3] += 10
	setPastEnd := ipfixtest.Message(1, ipfixtest.Set(flowTemplate, []byte{1, 2}))
	setPastEnd[19] += 10

	cases := map[string]struct {
		packet []byte
		want   error
	}{
		"short header":                {valid[:10], ErrNotIPFIX},
		"NetFlow v9":                  {v9, ErrNotIPFIX},
		"length past the datagram":    {longer, ErrTruncated},
		"set past the message":        {setPastEnd, ErrTruncated},
		"set shorter than its header": {ipfixtest.Message(1, []byte{0x01, 0x00, 0x00, 0x02}), ErrTruncated},
	}
	for name, c := range cases {
		_, err := NewTemplates().Decode(c.packet)
		assert.True(t, errors.Is(err, c.want), "%s: %v", name, err)
	}
}

func TestDecodeDropsTheHalfRecordAtTheEndOfASet(t *testing.T) {
	cut := append(bytes.Clone(oneFlow()), oneFlow()[:20]...)

	res := decodeAll(t, NewTemplates(), ipfixtest.Message(1,
		ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...), ipfixtest.Set(flowTemplate, cut)))

	assert.Len(t, res.Flows, 1)
}

func TestDecodeCapsTemplatesPerExporter(t *testing.T) {
	tmpl := NewTemplates()
	var sets [][]byte
	for i := 0; i <= MaxTemplatesPerExporter; i++ {
		sets = append(sets, ipfixtest.TemplateSet(uint16(minDataSetID+i), ipfixtest.FlowFields...))
	}

	res := decodeAll(t, tmpl, ipfixtest.Message(1, sets...))
	assert.Equal(t, 1, res.RejectedTemplates)

	again := decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(minDataSetID, ipfixtest.FlowFields...)))
	assert.Zero(t, again.RejectedTemplates, "re-sending a known template is not a new one")
}

func TestDecodeRejectsTemplatesWithImpossibleLengths(t *testing.T) {
	cases := map[string]ipfixtest.Field{
		"16-byte IPv4 address":    {ID: 8, Length: 16},
		"9-byte counter":          {ID: 1, Length: 9},
		"variable-length address": {ID: 12, Length: ipfixtest.VariableLength},
		"zero-length field":       {ID: 99, Length: 0},
	}
	for name, bad := range cases {
		tmpl := NewTemplates()
		fields := append([]ipfixtest.Field{bad}, ipfixtest.FlowFields...)
		res := decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate, fields...)))
		assert.Equal(t, 1, res.RejectedTemplates, name)

		data := decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.DataSet(flowTemplate, oneFlow())))
		assert.Equal(t, 1, data.MissingTemplate, name)
	}
}

// RouterOS sends IPv6 flows under their own template and describes itself in
// options records. Neither is IPv4 traffic, and neither is a missing template.
func TestDecodeSkipsIPv6AndOptionsRecordsQuietly(t *testing.T) {
	ipv6 := []ipfixtest.Field{{ID: 27, Length: 16}, {ID: 28, Length: 16}, {ID: 1, Length: 8}}
	ipv6Record := bytes.Join([][]byte{make([]byte, 32), ipfixtest.U64(100)}, nil)
	options := []ipfixtest.Field{{ID: 149, Length: 4}, {ID: 34, Length: 4}}
	optionsRecord := bytes.Join([][]byte{ipfixtest.U32(1), ipfixtest.U32(100)}, nil)

	res := decodeAll(t, NewTemplates(), ipfixtest.Message(1,
		ipfixtest.TemplateSet(257, ipv6...), ipfixtest.OptionsTemplateSet(258, 1, options...),
		ipfixtest.DataSet(257, ipv6Record), ipfixtest.DataSet(258, optionsRecord)))

	assert.Empty(t, res.Flows)
	assert.Zero(t, res.MissingTemplate)
	assert.Zero(t, res.RejectedTemplates)
}

func TestDecodeForgetsAWithdrawnTemplate(t *testing.T) {
	tmpl := NewTemplates()
	decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate, ipfixtest.FlowFields...)))
	decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.TemplateSet(flowTemplate)))

	res := decodeAll(t, tmpl, ipfixtest.Message(1, ipfixtest.DataSet(flowTemplate, oneFlow())))

	assert.Equal(t, 1, res.MissingTemplate)
}
