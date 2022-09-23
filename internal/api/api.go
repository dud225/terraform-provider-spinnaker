package api

import "github.com/spinnaker/spin/cmd/gateclient"

type Spinnaker struct {
  Client *gateclient.GatewayClient
}
