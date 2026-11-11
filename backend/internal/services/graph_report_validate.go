package services

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// validate checks a submitted report and writes its send time in canonical
// form, the default when left blank. current is the report as stored, nil
// for a new one.
func (s *GraphReportService) validate(in *GraphReportInput, current *models.GraphReport) error {
	if err := validateGraphReportFields(*in); err != nil {
		return err
	}
	in.SendTime = strings.TrimSpace(in.SendTime)
	if in.SendTime == "" {
		in.SendTime = DefaultGraphReportSendTime
	}
	if _, err := parseSendTime(in.SendTime); err != nil {
		return err
	}
	if in.WAGroupID == nil && in.TelegramChatID == nil {
		return fmt.Errorf("%w: pilih grup WhatsApp atau Telegram", ErrValidation)
	}
	if in.WAGroupID != nil {
		if err := s.validateWAGroup(*in.WAGroupID); err != nil {
			return err
		}
	}
	return s.validateTelegram(*in, current)
}

func (s *GraphReportService) validateWAGroup(id uuid.UUID) error {
	var group models.WAGroup
	err := s.db.Select("id").First(&group, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: grup WhatsApp tidak ditemukan", ErrValidation)
	}
	if err != nil {
		return fmt.Errorf("look up group: %w", err)
	}
	return nil
}

func validateGraphReportFields(in GraphReportInput) error {
	if err := requireText(in.CustomerName, "nama pelanggan"); err != nil {
		return err
	}
	if utf8.RuneCountInString(strings.TrimSpace(in.Message)) > MaxGraphReportMessageRunes {
		return fmt.Errorf("%w: pesan paling panjang %d karakter", ErrValidation, MaxGraphReportMessageRunes)
	}
	if len(in.Ports) == 0 || len(in.Ports) > MaxGraphReportPorts {
		return fmt.Errorf("%w: jumlah port harus 1 sampai %d", ErrValidation, MaxGraphReportPorts)
	}
	for i, port := range in.Ports {
		if port.DeviceID <= 0 {
			return fmt.Errorf("%w: port ke-%d belum memilih perangkat", ErrValidation, i+1)
		}
		if err := validateGraphReportPort(port, i+1); err != nil {
			return err
		}
	}
	return nil
}

func validateGraphReportPort(port GraphReportPortInput, n int) error {
	if utf8.RuneCountInString(strings.TrimSpace(port.DeviceName)) > graphReportTextRunes {
		return fmt.Errorf("%w: nama perangkat port ke-%d paling panjang %d karakter", ErrValidation, n, graphReportTextRunes)
	}
	if err := requireText(port.IfName, fmt.Sprintf("nama interface port ke-%d", n)); err != nil {
		return err
	}
	return requireText(port.Label, fmt.Sprintf("label port ke-%d", n))
}

func requireText(value, name string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%w: %s wajib diisi", ErrValidation, name)
	}
	if utf8.RuneCountInString(trimmed) > graphReportTextRunes {
		return fmt.Errorf("%w: %s paling panjang %d karakter", ErrValidation, name, graphReportTextRunes)
	}
	return nil
}
