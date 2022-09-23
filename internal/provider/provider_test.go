package provider

import (
	"fmt"
	"encoding/json"
  "net/http"
  "net/http/httptest"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"

  "gopkg.in/yaml.v3"
)

var protoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"spinnaker": providerserver.NewProtocol6WithError(New("test")()),
}

func TestProviderConfigure(t *testing.T) {
	ts := mockSpinnaker()
	defer ts.Close()
  spinConfig, err := writeSpinConfig(ts.URL)
  if err != nil {
    t.Fatalf(err.Error())
  }
	defer os.Remove(spinConfig.Name())

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: providerConfig(spinConfig),
			},
		},
	})
}

func providerConfig(spinConfig *os.File) string {
  return fmt.Sprintf(`
provider "spinnaker" {
	spin_config = %q
  default_headers = "Unit-Test=true"
	ignore_cert_errors = true
}

data spinnaker_application "test" {
  name = "testapp"
}
	`, spinConfig.Name())
}

func checkHTTPHeader(next http.Handler) http.Handler {
  return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.Header.Get("Unit-Test") != "true" {
			http.Error(w, "Missing header Unit-Test or incorrect value", http.StatusBadRequest)
			return
		}
    next.ServeHTTP(w, r)
  })
}

func mockSpinnaker() (*httptest.Server) {
	mux := http.NewServeMux()
  mux.Handle("/version", checkHTTPHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(map[string]string{"version": "test"})
		w.Header().Add("content-type", "application/json")
		fmt.Fprintln(w, string(b))
  })))
  mux.Handle("/applications/testapp", checkHTTPHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(map[string]interface{}{
			"attributes": map[string]interface{}{
				"cloudProviders": "",
				"email": "test@example.com",
				"name": "testapp",
			},
		})
		w.Header().Add("content-type", "application/json")
		fmt.Fprintln(w, string(b))
  })))

  return httptest.NewTLSServer(mux)
}

func writeSpinConfig(spinURL string) (*os.File, error) {
  spinConfig, err := os.CreateTemp("", "spin_config")
	if err != nil {
		return nil, err
	}
  spinConfiguration, err := yaml.Marshal(map[string]interface{}{
    "gate": map[string]interface{}{
      "endpoint": spinURL,
    },
  })
  if err != nil {
		return nil, err
	}
  if err = os.WriteFile(spinConfig.Name(), []byte(spinConfiguration), 0444); err != nil {
		return nil, err
	}
	if err = spinConfig.Close(); err != nil {
		return nil, err
	}
  return spinConfig, nil
}
