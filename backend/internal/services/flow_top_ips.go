package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
)

const (
	// topIPRetention matches router_flow_top_ips' retention policy.
	topIPRetention = 7 * 24 * time.Hour
	// topIPRangeSlack lets the 7-day preset through: its start is computed in
	// the browser a moment before this check runs.
	topIPRangeSlack = 5 * time.Minute
	topIPLimit      = 100

	// countryFromServer marks a country DB-IP located the address in.
	countryFromServer = "server"
	// countryFromAS marks the AS's registered country, shown when the range
	// holds only windows written before flowd located addresses.
	countryFromAS = "as"
)

// ErrTopIPsRange refuses a top-IP read reaching before what the table keeps.
var ErrTopIPsRange = fmt.Errorf("%w: top IPs are kept for 7 days; the range starts before that", ErrValidation)

// FlowTopIP is one destination address over the range.
type FlowTopIP struct {
	IP   string `json:"ip" gorm:"column:ip"`
	ASN  int64  `json:"asn" gorm:"column:asn"`
	Name string `json:"name" gorm:"-"`
	// Country is where the address is located (DB-IP), "" when unknown or
	// for windows written before flowd recorded it.
	Country string `json:"country" gorm:"column:country"`
	// CountrySource is countryFromServer, countryFromAS, or "" with no country.
	CountrySource string `json:"country_source" gorm:"-"`
	Bytes         int64  `json:"bytes"`
	// Flows sums the sampled flow records over the range.
	Flows int64 `json:"flows"`
}

// FlowStatus describes the ASN dataset flowd last loaded.
type FlowStatus struct {
	ASNDatasetUpdatedAt *time.Time `json:"asn_dataset_updated_at"`
	ASNCount            int64      `json:"asn_count"`
}

func checkTopIPsRange(from, now time.Time) error {
	if now.Sub(from) > topIPRetention+topIPRangeSlack {
		return ErrTopIPsRange
	}
	return nil
}

// TopIPs sums each address's windows over the range. A window kept only its
// top addresses, so one just under the cut in some windows is undercounted
// there: this ranks heavy hitters, it does not total them.
func (s *FlowQueryService) TopIPs(routerID uuid.UUID, f FlowFilter, now time.Time) ([]FlowTopIP, error) {
	if err := checkTopIPsRange(f.From, now); err != nil {
		return nil, err
	}
	f.Upstream = nil // the table has no upstream column
	where, args := f.where(routerID, "time")
	// MAX picks the located country over "" from windows that predate it.
	query := fmt.Sprintf(`SELECT host(ip) AS ip, asn, MAX(country) AS country, %s AS bytes,
		%s AS flows FROM router_flow_top_ips
		WHERE %s GROUP BY ip, asn ORDER BY bytes DESC LIMIT ?`, cappedBigint("SUM(bytes)"), cappedBigint("SUM(flows)"), where)
	rows := []FlowTopIP{}
	if err := s.db.Raw(query, withArgs(args, topIPLimit)...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	asns := make([]int64, len(rows))
	for i, r := range rows {
		asns[i] = r.ASN
	}
	records, err := s.asnRecords(asns)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(records))
	for asn, r := range records {
		names[asn] = r.Name
	}
	for i := range rows {
		rows[i].Name = asnLabel(rows[i].ASN, names)
		placeCountry(&rows[i], records[rows[i].ASN].Country)
	}
	return rows, nil
}

// placeCountry keeps the located country, or falls back to the AS's
// registered one, which iptoasn gives as "Unknown" when it has none.
func placeCountry(row *FlowTopIP, asCountry string) {
	switch {
	case row.Country != "":
		row.CountrySource = countryFromServer
	case isCountryCode(asCountry):
		row.Country, row.CountrySource = asCountry, countryFromAS
	}
}

func isCountryCode(s string) bool {
	return len(s) == 2 && s[0] >= 'A' && s[0] <= 'Z' && s[1] >= 'A' && s[1] <= 'Z' && s != "ZZ"
}

// Status reports the newest dataset's download time and how many ASes it
// names. flowd writes every name with its dataset's time, so the newest row
// dates the dataset in use.
func (s *FlowQueryService) Status() (*FlowStatus, error) {
	var count int64
	if err := s.db.Model(&models.ASNName{}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count == 0 {
		return &FlowStatus{}, nil
	}
	var newest models.ASNName
	if err := s.db.Order("updated_at DESC").First(&newest).Error; err != nil {
		return nil, err
	}
	return &FlowStatus{ASNDatasetUpdatedAt: &newest.UpdatedAt, ASNCount: count}, nil
}
