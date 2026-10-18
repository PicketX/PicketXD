// SPDX-License-Identifier: GPL-3.0-or-later
package hostagent

import (
	"github.com/PicketX/PicketX/pkg/configuration"
	"testing"
)

// Runtime facts use numeric family arrays, as documented by the Controller.
// Direct json.Marshal(networkplan.CapabilityProfile) encodes []Family (uint8)
// as base64; use the external document representation at this boundary.
func controllerProcessFactsDocument() []byte {
	return []byte(`{"authorityID":"static-test","agents":[{"Name":"node","Labels":{},"Capabilities":{"Classes":["host-local"],"Protocols":["tcp","udp"],"Families":[4,6],"Handling":["graceful"],"OriginalDestination":true}}],"contexts":[{"AgentRef":"node","ProfileRef":"host","ID":"controller-context","NetworkSpaceID":"declarative-not-local","Class":"host-local","Stage":"original-destination","Handling":"graceful","Destinations":["127.0.0.1/32","::1/128"]}]}`)
}
func TestControllerProcessFactsStrictRoundTrip(t *testing.T) {
	f, e := configuration.DecodeFacts(controllerProcessFactsDocument())
	if e != nil {
		t.Fatal(e)
	}
	b, e := configuration.Decode([]byte("apiVersion: picketx.io/v1alpha1\nkind: Configuration\nmetadata: {name: empty, uid: empty-v1}\nspec: {empty: true}\n"))
	if e != nil {
		t.Fatal(e)
	}
	if _, e = configuration.Compile(b, f); e != nil {
		t.Fatal(e)
	}
}
