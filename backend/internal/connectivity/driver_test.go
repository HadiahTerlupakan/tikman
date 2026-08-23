package connectivity

import (
	"errors"
	"strings"
	"testing"

	"github.com/tikman/olt-provisioning/internal/models"
)

func TestDriverForResolvesEveryRegisteredModel(t *testing.T) {
	for _, model := range []models.OLTModel{
		models.OLTModelZTEC300,
		models.OLTModelZTEC320,
		models.OLTModelHSGQ,
	} {
		driver, err := DriverFor(model)
		if err != nil {
			t.Fatalf("DriverFor(%q): %v", model, err)
		}
		if driver.Model() != model {
			t.Errorf("DriverFor(%q) returned a driver for %q", model, driver.Model())
		}
	}
}

// An unknown model must not silently resolve to ZTE: applying one vendor's
// decoding to another's raw values yields readings that look plausible and are
// wrong, which is worse than a monitoring gap.
func TestDriverForRejectsUnknownModelWithoutFallback(t *testing.T) {
	driver, err := DriverFor(models.OLTModel("huawei_ma5800"))
	if err == nil {
		t.Fatalf("expected an error, got driver for %q", driver.Model())
	}
	if driver != nil {
		t.Errorf("got driver %q, want nil", driver.Model())
	}
	if !strings.Contains(err.Error(), "huawei_ma5800") {
		t.Errorf("error %q does not name the rejected model", err)
	}
}

func TestDriverForRejectsEmptyModel(t *testing.T) {
	if _, err := DriverFor(""); err == nil {
		t.Fatal("expected an error for an unset model")
	}
}

func TestSupportedModelsListsRegisteredDrivers(t *testing.T) {
	got := SupportedModels()
	want := []models.OLTModel{models.OLTModelZTEC300, models.OLTModelZTEC320, models.OLTModelHSGQ}

	if len(got) != len(want) {
		t.Fatalf("SupportedModels() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("SupportedModels()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestUnsupportedReadPathsReportErrUnsupported(t *testing.T) {
	driver, err := DriverFor(models.OLTModelHSGQ)
	if err != nil {
		t.Fatalf("DriverFor: %v", err)
	}

	if _, err := driver.WalkTrafficRates("127.0.0.1", "public", 161); !errors.Is(err, ErrUnsupported) {
		t.Errorf("WalkTrafficRates error = %v, want ErrUnsupported", err)
	}
	if _, err := driver.QueryTrafficRates("127.0.0.1", "public", 161, 1, 1, 1); !errors.Is(err, ErrUnsupported) {
		t.Errorf("QueryTrafficRates error = %v, want ErrUnsupported", err)
	}
}
