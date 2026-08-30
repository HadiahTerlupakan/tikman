package services

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
	"gorm.io/gorm"
)

// OLTService handles OLT operations
type OLTService struct {
	db               *gorm.DB
	encryptionKey    []byte
	commanderFactory CommanderFactory
}

// NewOLTServiceWithCommander adds CLI access, which the discovery poll uses to
// read profile lists the OLT does not publish over SNMP. Without a factory the
// service still works and simply skips them.
func NewOLTServiceWithCommander(db *gorm.DB, encryptionKey string, factory CommanderFactory) *OLTService {
	service := NewOLTService(db, encryptionKey)
	service.commanderFactory = factory
	return service
}

// NewOLTService creates a new OLT service
func NewOLTService(db *gorm.DB, encryptionKey string) *OLTService {
	return &OLTService{
		db:            db,
		encryptionKey: []byte(encryptionKey),
	}
}

// GetDB returns the database instance
func (s *OLTService) GetDB() *gorm.DB {
	return s.db
}

// GetEncryptionKey returns the encryption key
func (s *OLTService) GetEncryptionKey() []byte {
	return s.encryptionKey
}

// Create creates a new OLT with status validation
// CreateOLTInput carries everything needed to register an OLT. It is a struct
// rather than a parameter list because the list had reached fourteen, six of
// them consecutive ints — rack, shelf, slot and three ports — where swapping
// two at a call site compiles cleanly and fails only against real hardware.
type CreateOLTInput struct {
	SiteID            uuid.UUID
	Name              string
	IPAddress         string
	SNMPCommunity     string
	Username          string
	Password          string
	Model             models.OLTModel
	SSHPort           int
	TelnetPort        int
	SNMPPort          int
	PreferredProtocol models.OLTProtocol
	// Rack, shelf and slot are deliberately absent: the previous signature took
	// them and never used them, because discovery resolves the physical
	// position at ONT level. Carrying them here would restate that mistake.
	Latitude  *float64
	Longitude *float64
}

// Create registers an OLT.
func (s *OLTService) Create(in CreateOLTInput) (*models.OLT, error) {
	siteID := in.SiteID
	name, ipAddress, snmpCommunity := in.Name, in.IPAddress, in.SNMPCommunity
	username, password := in.Username, in.Password
	model := in.Model
	sshPort, telnetPort, snmpPort := in.SSHPort, in.TelnetPort, in.SNMPPort
	preferredProtocol := in.PreferredProtocol

	if err := validateCoordinates(in.Latitude, in.Longitude); err != nil {
		return nil, err
	}

	// A model with no driver would leave the OLT unmonitorable, so it is
	// rejected here as well as at the API boundary.
	if _, err := connectivity.DriverFor(model); err != nil {
		return nil, err
	}

	// Validate SNMP port
	if snmpPort < 1 || snmpPort > 65535 {
		return nil, fmt.Errorf("invalid SNMP port: %d", snmpPort)
	}

	// Validate SSH port
	if sshPort < 1 || sshPort > 65535 {
		return nil, fmt.Errorf("invalid SSH port: %d", sshPort)
	}

	// Validate Telnet port
	if telnetPort < 1 || telnetPort > 65535 {
		return nil, fmt.Errorf("invalid Telnet port: %d", telnetPort)
	}

	// Encrypt password before storing using encryptionKey as string
	encryptedPassword, err := utils.Encrypt(password, strings.TrimSpace(string(s.encryptionKey)))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt password: %w", err)
	}

	// The site has to exist. This used to be uuid.New(), so every OLT created
	// through the API pointed at a site that was never there: the list showed a
	// blank Site column and any per-site query silently matched nothing. Checked
	// after the free input validation above and before the SNMP probe, so the
	// database is only touched once the request is structurally sound.
	var site models.Site
	if err := s.db.First(&site, "id = ?", siteID).Error; err != nil {
		return nil, fmt.Errorf("site not found: %w", err)
	}

	// Validate SNMP connectivity if community provided
	if snmpCommunity != "" {
		if err := connectivity.SNMPTest(ipAddress, snmpPort, snmpCommunity, 0); err != nil {
			return nil, fmt.Errorf("SNMP connection test failed: %w", err)
		}
	}

	// Create OLT without Rack/Shelf/Slot - discovery happens at ONT level
	olt := &models.OLT{
		SiteID:            siteID,
		Name:              name,
		IPAddress:         ipAddress,
		SSHPort:           sshPort,
		TelnetPort:        telnetPort,
		SNMPPort:          snmpPort,
		SNMPCommunity:     snmpCommunity,
		Latitude:          in.Latitude,
		Longitude:         in.Longitude,
		PreferredProtocol: preferredProtocol,
		Model:             model,
		Username:          username,
		Password:          encryptedPassword,
		Status:            models.OLTStatusOnline, // Changed from Offline to Online
	}

	if err := s.db.Create(olt).Error; err != nil {
		return nil, fmt.Errorf("failed to create OLT: %w", err)
	}

	return olt, nil
}

func (s *OLTService) GetByID(id uuid.UUID) (*models.OLT, error) {
	var olt models.OLT
	if err := s.db.First(&olt, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("OLT not found: %w", err)
	}
	return &olt, nil
}

func (s *OLTService) List() ([]models.OLT, error) {
	var olts []models.OLT
	if err := s.db.Find(&olts).Error; err != nil {
		return nil, fmt.Errorf("failed to list OLTs: %w", err)
	}
	return olts, nil
}

func (s *OLTService) Update(id uuid.UUID, updates map[string]interface{}) error {
	// Shares validateCoordinates with SiteService: the rule is a property of a
	// coordinate pair, not of the thing carrying it, so it lives in one place.
	latitude, hasLatitude := updates["latitude"].(*float64)
	longitude, hasLongitude := updates["longitude"].(*float64)
	if hasLatitude || hasLongitude {
		if err := validateCoordinates(latitude, longitude); err != nil {
			return err
		}
	}

	if password, ok := updates["password"].(string); ok {
		encryptedPassword, err := utils.Encrypt(password, strings.TrimSpace(string(s.encryptionKey)))
		if err != nil {
			return fmt.Errorf("failed to encrypt password: %w", err)
		}
		updates["password"] = encryptedPassword
	}

	if err := s.db.Model(&models.OLT{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update OLT: %w", err)
	}

	return nil
}

// Delete removes an OLT and all dependent data in one transaction: ONTs,
// their metrics, and their events. Without this cleanup, deleting an OLT
// left orphaned ONT rows that still showed up in listings.
func (s *OLTService) Delete(id uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var ontIDs []uuid.UUID
		if err := tx.Model(&models.ONT{}).Where("olt_id = ?", id).Pluck("id", &ontIDs).Error; err != nil {
			return fmt.Errorf("failed to list ONTs for OLT: %w", err)
		}

		if len(ontIDs) > 0 {
			if tx.Migrator().HasTable("ont_metrics") {
				for _, ontID := range ontIDs {
					if err := tx.Exec("DELETE FROM ont_metrics WHERE ont_id = ?", ontID).Error; err != nil {
						return fmt.Errorf("failed to delete ONT metrics: %w", err)
					}
				}
			}
		}
		if len(ontIDs) > 0 {
			if err := tx.Where("ont_id IN ?", ontIDs).Delete(&models.ONTEvent{}).Error; err != nil {
				return fmt.Errorf("failed to delete ONT events: %w", err)
			}
		}
		if err := tx.Delete(&models.ONT{}, "olt_id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete ONTs: %w", err)
		}

		result := tx.Delete(&models.OLT{}, "id = ?", id)
		if result.Error != nil {
			return fmt.Errorf("failed to delete OLT: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("OLT not found")
		}
		return nil
	})
}

// DiscoverONTs discovers all ONTs connected to this OLT via SNMP topology walk
func (s *OLTService) DiscoverONTs(oltID uuid.UUID) ([]connectivity.DiscoveredONT, error) {
	// Get OLT details
	olt, err := s.GetByID(oltID)
	if err != nil {
		return nil, fmt.Errorf("OLT not found: %w", err)
	}

	// Decrypt SNMP community
	snmpCommunity := olt.SNMPCommunity
	if snmpCommunity == "" {
		return nil, fmt.Errorf("SNMP community not configured for this OLT")
	}

	// Perform discovery using topology-based approach
	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		return nil, err
	}

	topology, err := connectivity.DiscoverOLTTopology(driver, olt.IPAddress, snmpCommunity, olt.SNMPPort)
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}

	// Flatten topology to simple list of discovered ONTs
	var result []connectivity.DiscoveredONT
	for _, slot := range topology {
		for _, gponPort := range slot.Ports {
			result = append(result, gponPort.ONTs...)
		}
	}

	return result, nil
}

// SiteNameForOLT resolves the site name for an OLT row. A missing or unset
// site yields an empty string rather than an error, mirroring the previous
// in-DTO behaviour.
func (s *OLTService) SiteNameForOLT(siteID uuid.UUID) string {
	if siteID == uuid.Nil {
		return ""
	}
	var site models.Site
	if err := s.db.Where("id = ?", siteID).First(&site).Error; err == nil {
		return site.Name
	}
	return ""
}

// staleDiscoveryClaim is how long a discovery may go without publishing
// progress before another run may take its claim over. The claim lives in the
// database but the run holding it does not: a restart mid-walk leaves the
// phase at "discovering" with nobody working on it, and without this the OLT
// would never be polled again.
//
// It is measured against the heartbeat, not the start, so it only has to
// exceed the longest quiet stretch inside a run rather than the whole run.
// Storing the metrics for a large OLT is that stretch, at well under a minute.
const staleDiscoveryClaim = 5 * time.Minute

// minDiscoveryInterval is the quiet period after a completed discovery before
// another one may start. The worker ticks every minute but a full walk of a
// populated OLT takes several, so without this the run that just finished was
// restarted on the next tick: the progress bar reset to zero over and over and
// never showed a completed poll, and the OLT was walked continuously. ONT
// status does not depend on this — the metrics cycle walks it separately every
// minute.
const minDiscoveryInterval = 5 * time.Minute

// TryClaimDiscovery atomically claims an OLT for one discovery run. The
// database claim prevents the API-triggered run and worker fallback from
// polling the same OLT concurrently, and holds off a run that would follow a
// completed one too closely. An OLT that has never been polled has no last
// poll time, so a newly created one is discovered immediately.
func (s *OLTService) TryClaimDiscovery(oltID uuid.UUID) (bool, error) {
	// A run that has never published progress is judged on when it started, so
	// a claim taken but abandoned before its first instalment still expires.
	liveness := "COALESCE(discovery_heartbeat_at, discovery_started_at)"

	result := s.db.Model(&models.OLT{}).
		Where("id = ? AND (discovery_phase NOT IN ? OR "+liveness+" IS NULL OR "+liveness+" < ?)",
			oltID, []string{"discovering", "polling"}, time.Now().Add(-staleDiscoveryClaim)).
		Where("discovery_last_poll_at IS NULL OR discovery_last_poll_at < ?",
			time.Now().Add(-minDiscoveryInterval)).
		Updates(map[string]interface{}{
			"discovery_phase": "discovering", "discovery_error": "",
			// Stamped here, not when the walk starts, so the claim itself carries
			// the age the takeover above tests against.
			"discovery_started_at":   time.Now(),
			"discovery_heartbeat_at": time.Now(),
		})
	if result.Error != nil {
		return false, fmt.Errorf("claim discovery: %w", result.Error)
	}
	return result.RowsAffected == 1, nil
}

// AutoDiscoverONTMetrics polls ONT metrics from this OLT via SNMP and stores
// them asynchronously. The handler and worker may both call it; the database
// claim ensures only one run proceeds.
func (s *OLTService) AutoDiscoverONTMetrics(olt *models.OLT) {
	claimed, err := s.TryClaimDiscovery(olt.ID)
	if err != nil {
		log.Printf("[AutoDiscovery] Cannot claim OLT %s: %v", olt.Name, err)
		return
	}
	if !claimed {
		log.Printf("[AutoDiscovery] Skipping OLT %s: discovery already running", olt.Name)
		return
	}
	// The metrics cycle walks the same tables. Whoever gets here second stands
	// down rather than doubling the load on the OLT.
	release, free := TryLockOLTSNMP(olt.ID)
	if !free {
		log.Printf("[AutoDiscovery] Skipping OLT %s: another collector is reading it", olt.Name)
		s.updateDiscoveryProgress(olt.ID, map[string]interface{}{"discovery_phase": "idle"})
		return
	}
	defer release()

	log.Printf("[AutoDiscovery] Starting immediate ONT metrics polling for OLT %s (%s)", olt.Name, olt.IPAddress)

	start := time.Now()
	s.updateDiscoveryProgress(olt.ID, map[string]interface{}{
		"discovery_phase": "discovering", "discovery_total": 0,
		"discovery_registered": 0, "discovery_polled": 0,
		"discovery_error": "", "discovery_started_at": start,
	})

	metricsService := NewMetricsService(s.db)

	driver, err := connectivity.DriverFor(olt.Model)
	if err != nil {
		log.Printf("[AutoDiscovery] Cannot poll OLT %s: %v", olt.Name, err)
		return
	}

	s.refreshVLANCache(olt)
	s.refreshSystemCache(olt)
	s.refreshProfileCache(olt)

	// Enumerate ONTs before the slower optical-metrics walk so the UI can show
	// a discovery total as soon as the OLT reports its status table.
	statuses, statusErr := driver.WalkStatuses(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	if statusErr != nil {
		log.Printf("[AutoDiscovery] Status walk failed for OLT %s: %v", olt.Name, statusErr)
	}
	locations := make(map[connectivity.ONTLocation]bool, len(statuses))
	for loc := range statuses {
		locations[loc] = true
	}
	if len(locations) > 0 {
		s.updateDiscoveryProgress(olt.ID, map[string]interface{}{
			"discovery_phase": "discovering", "discovery_total": len(locations),
		})
	}

	// Read for the ONUs the status walk named, rather than swept: sweeping the
	// optical tables does not finish on a populated OLT, and returned 96 of 200
	// rows before timing out. A failure here must still not prevent registering
	// ONTs from status data.
	allMetrics, metricsErr := readONTMetrics(driver, olt, locations)
	if metricsErr != nil {
		log.Printf("[AutoDiscovery] Metrics read failed for OLT %s: %v; continuing with inventory", olt.Name, metricsErr)
		allMetrics = make(map[connectivity.ONTLocation]connectivity.ONTMetrics)
	}
	for loc := range allMetrics {
		locations[loc] = true
	}
	for loc := range allMetrics {
		locations[loc] = true
	}
	for loc := range statuses {
		locations[loc] = true
	}
	log.Printf("[AutoDiscovery] Found %d ONT positions via SNMP", len(locations))
	s.updateDiscoveryProgress(olt.ID, map[string]interface{}{
		"discovery_phase": "polling", "discovery_total": len(locations),
	})

	locationList := make([]connectivity.ONTLocation, 0, len(locations))
	for loc := range locations {
		locationList = append(locationList, loc)
	}

	// Registering each instalment as the walk reports it, rather than after the
	// whole inventory, is what lets the progress bar move on a large OLT.
	processed := 0
	inventoryErr := driver.InventoryByPort(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, locationList,
		func(locs []connectivity.ONTLocation, inventory map[connectivity.ONTLocation]connectivity.ONTInventory) {
			processed += len(locs)
			s.registerDiscoveredONTs(olt, locs, inventory, statuses, allMetrics, processed)
		})
	if inventoryErr != nil {
		log.Printf("[AutoDiscovery] Inventory walk failed for OLT %s: %v", olt.Name, inventoryErr)
	}
	registered, _ := NewONTService(s.db).CountONTsByOLT(olt.ID)
	if registered == 0 && len(locationList) > 0 {
		s.updateDiscoveryProgress(olt.ID, map[string]interface{}{"discovery_error": "inventory unavailable"})
	}

	var successCount int

	for loc, metrics := range allMetrics {
		var onts []models.ONT
		if err := s.db.Where("olt_id = ? AND port_id = ? AND ont_id = ?", olt.ID, loc.Port, loc.ONTID).Find(&onts).Error; err != nil {
			continue
		}

		if len(onts) == 0 {
			log.Printf("[AutoDiscovery] Skipping unregistered ONT at port=%d ont=%d", loc.Port, loc.ONTID)
			continue
		}

		ont := onts[0]

		storeMetrics := false
		if metrics.RxPower != nil || metrics.TxPower != nil || metrics.Distance > 0 {
			storeMetrics = true
		}

		if storeMetrics {
			if err := metricsService.StoreMetrics(ont.ID, &metrics, nil); err != nil {
				log.Printf("[AutoDiscovery] Failed to store metrics for ONT %s: %v", ont.SerialNumber, err)
				continue
			}
			rxStr := "-"
			if metrics.RxPower != nil {
				rxStr = fmt.Sprintf("%.2f", *metrics.RxPower)
			}
			txStr := "-"
			if metrics.TxPower != nil {
				txStr = fmt.Sprintf("%.2f", *metrics.TxPower)
			}
			log.Printf("[AutoDiscovery] ✅ Polled metrics: serial=%s port=%d/%d rx_power=%s dBm tx_power=%s dBm distance=%dm",
				ont.SerialNumber, loc.Port, loc.ONTID, rxStr, txStr, metrics.Distance)
			successCount++
		}
	}

	s.updateDiscoveryProgress(olt.ID, map[string]interface{}{
		"discovery_phase": "completed", "discovery_polled": successCount,
		"discovery_last_poll_at": time.Now(),
	})
	log.Printf("[AutoDiscovery] Completed: polled metrics for %d ONTs from OLT %s", successCount, olt.Name)
}

// registerDiscoveredONTs persists one instalment of an inventory walk and
// publishes how far the walk has got. Progress counts locations this walk has
// covered, not rows in the table: re-running discovery on an OLT that is
// already populated would otherwise show a full bar from the first instalment.
func (s *OLTService) registerDiscoveredONTs(
	olt *models.OLT,
	locs []connectivity.ONTLocation,
	inventory map[connectivity.ONTLocation]connectivity.ONTInventory,
	statuses map[connectivity.ONTLocation]int,
	allMetrics map[connectivity.ONTLocation]connectivity.ONTMetrics,
	processed int,
) {
	registerService := NewONTService(s.db)

	discovered := make([]connectivity.DiscoveredONT, 0, len(locs))
	for _, loc := range locs {
		item := connectivity.DiscoveredONT{Slot: loc.Slot, PortID: loc.Port, ONTID: loc.ONTID, RunState: statuses[loc]}
		if inv, ok := inventory[loc]; ok {
			item.SerialNumber = inv.SerialNumber
			item.Name = inv.Name
			item.DeviceType = inv.DeviceType
			item.HardwareVersion = inv.HardwareVersion
			item.IPAddress = inv.IPAddress
		}
		if metrics, ok := allMetrics[loc]; ok {
			item.RxPower = metrics.RxPower
			item.TxPower = metrics.TxPower
			item.Distance = metrics.Distance
		}
		discovered = append(discovered, item)
	}

	result := registerService.BulkRegisterFromDiscovery(olt.ID, discovered)
	s.updateDiscoveryProgress(olt.ID, map[string]interface{}{"discovery_registered": processed})

	slot, port := 0, 0
	if len(locs) > 0 {
		slot, port = locs[0].Slot, locs[0].Port
	}
	log.Printf("[AutoDiscovery] Instalment slot=%d port=%d: onts=%d touched=%d processed=%d", slot, port, len(locs), result.Registered, processed)
}

func (s *OLTService) updateDiscoveryProgress(oltID uuid.UUID, updates map[string]interface{}) {
	// Every progress publish doubles as the claim's heartbeat, so liveness costs
	// no extra write and cannot drift from the work actually being done.
	updates["discovery_heartbeat_at"] = time.Now()
	if err := s.db.Model(&models.OLT{}).Where("id = ?", oltID).Updates(updates).Error; err != nil {
		log.Printf("[AutoDiscovery] Failed to update progress for OLT %s: %v", oltID, err)
	}
}

// readONTMetrics prefers a driver that can read named ONUs, falling back to the
// table sweep for one that cannot.
func readONTMetrics(driver connectivity.Driver, olt *models.OLT, known map[connectivity.ONTLocation]bool) (map[connectivity.ONTLocation]connectivity.ONTMetrics, error) {
	querier, direct := driver.(connectivity.MetricsQuerier)
	if !direct || len(known) == 0 {
		return driver.WalkMetrics(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort)
	}

	locations := make([]connectivity.ONTLocation, 0, len(known))
	for loc := range known {
		locations = append(locations, loc)
	}
	return querier.QueryMetricsFor(olt.IPAddress, olt.SNMPCommunity, olt.SNMPPort, locations)
}
