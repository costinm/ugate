package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/costinm/meshauth"
	"github.com/costinm/meshauth/pkg/tokens"
	"github.com/costinm/ugate/appinit"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"

	//"sigs.k8s.io/yaml"

	k8sc "github.com/costinm/mk8s"
)

// To avoid a yaml dependency, run:
// yq < ~/.kube/config -o json > ~/.kube/config.json
// See examples for additional configurations needed for the cluster.
// The tests should be run with a kube config pointing to a GKE cluster with the required configs.

func TestResourceStore(t *testing.T) {

	b := os.DirFS("../testdata/ca")
	ctx := context.Background()
	cs := appinit.AppResourceStore()

	err := cs.Load(ctx, b, "../testdata/ca")
	if err != nil {
		panic(err)
	}

	err = cs.Start()
	if err != nil {
		panic(err)
	}

	// yaml can't really unmarshall json structs if they have RawJson
	csb, err := json.Marshal(cs)

	raw := map[string]any{}
	err = json.Unmarshal(csb, &raw)
	if err != nil {
		panic(err)
	}
	//csb, err = yaml.Marshal(raw)
	//if err != nil {
	//	panic(err)
	//}
	//t.Log(string(csb))

}

func checkJWT(t *testing.T, jwt string) {

	r, _ := http.NewRequest("GET", "http://example", nil)
	// Expired key - issue a new one
	r.Header["Authorization"] = []string{"bearer " + jwt}

	// Example of google JWT in cloudrun:
	// eyJhbGciOiJSUzI1NiIsImtpZCI6IjBlNzJkYTFkZjUwMWNhNmY3NTZiZjEwM2ZkN2M3MjAyOTQ3NzI1MDYiLCJ0eXAiOiJKV1QifQ.eyJpc3MiOiJodHRwczovL2FjY291bnRzLmdvb2dsZS5jb20iLCJhenAiOiIzMjU1NTk0MDU1OS5hcHBzLmdvb2dsZXVzZXJjb250ZW50LmNvbSIsImF1ZCI6IjMyNTU1OTQwNTU5LmFwcHMuZ29vZ2xldXNlcmNvbnRlbnQuY29tIiwic3ViIjoiMTA0MzY2MjYxNjgxNjMwMTM4NTIzIiwiZW1haWwiOiJjb3N0aW5AZ21haWwuY29tIiwiZW1haWxfdmVyaWZpZWQiOnRydWUsImF0X2hhc2giOiJ1MTIwMzhrTTh2THcyZGN0dnVvbTdBIiwiaWF0IjoxNzAwODg0MDAwLCJleHAiOjE3MDA4ODc2MDB9.SIGNATURE_REMOVED_BY_GOOGLE"

	cfg := &tokens.AuthnConfig{}

	cfg.Issuers = []*tokens.TrustConfig{{Issuer: "https://accounts.google.com"}}

	ja := cfg.New()

	// May use a custom method too with lower deps
	//ja.Verify = oidc.Verify

	_, err := ja.Auth(r)
	if err != nil {
		t.Fatal(err)
	}
}

func TestGKE(t *testing.T) {
	ctx := context.Background()

	// Start with loading the current K8S
	k8s, err := k8sc.New(ctx, "", "")

	if err != nil {
		t.Fatal(err)
	}
	if k8s.Default != nil {
		t.Log("Primary cluster", k8s.Default.Name)

	}

}

type TestModule struct {
	appCtx context.Context

	Path string

	Config map[string]any `json:"cfg,inline""`

	// Doesn't really work with std json package
	Extra interface{} `json:",inline"`
}

func TestCEL(t *testing.T) {
	terr := appinit.NewSlogError("cel", "example", "bar")

	// AnyTime = protobuf.Any

	ce, err := cel.NewEnv(
		cel.Variable("e", cel.DynType),
		cel.Variable("m", cel.StringType),
		cel.Variable("f", cel.StringType),
		cel.Function("testf",
			cel.MemberOverload("string_greet_string", []*cel.Type{cel.StringType, cel.StringType}, cel.StringType,
				cel.BinaryBinding(func(lhs, rhs ref.Val) ref.Val {
					return types.String(fmt.Sprintf("Hello %s! Nice to meet you, I'm %s.\n", rhs, lhs))
				}),
			)))
	if err != nil {
		t.Fatal(err)
	}

	// Protobuf form
	// type(time.Time) == google.protobuf.Timestamp
	ast, iss := ce.Compile(`{"e": e.message, "t": e.time}`)
	if iss.Err() != nil {
		t.Fatal(iss.Err())
	}
	prg, err := ce.Program(ast)
	if err != nil {
		t.Fatal(err)
	}

	out, _, err := prg.Eval(map[string]any{
		// Native values are converted to CEL values under the covers.
		"m": "CEL",
		"e": appinit.ErrorToMap(terr),
		// Values may also be lazily supplied.
		"f": func() ref.Val { return types.String("world") },
	})

	t.Log(out, err)
}

// Test starting with K8S credentials
// On a pod or a VM/dev with a kubeconfig file.
func TestK8SLite(t *testing.T) {
	ctx := context.Background()

	// Bootstrap K8S - get primary and secondary clusters.
	def, err := k8sc.New(ctx, "", "")
	if err != nil {
		t.Skip("Can't find a kube config file", err)
	}

	t.Run("K8S istio-ca tokens", func(t *testing.T) {
		// Will use the namespace/ksa from the config
		istiocaTok, err := def.GetToken(ctx, "Foo")
		if err != nil {
			t.Fatal(err)
		}
		_, istiocaT, _, _, _ := meshauth.JwtRawParse(istiocaTok)
		t.Log(istiocaT)
		if istiocaT != nil {
			t.Log(string(istiocaT.Payload))
		}
	})

	t.Run("K8S audience tokens", func(t *testing.T) {
		// Without audience overide - K8SCluster is a TokenSource as well
		tok, err := def.GetToken(ctx, "http://example.com")
		if err != nil {
			t.Error("Getting tokens with audience from k8s", err)
		}

		_, tokT, _, _, _ := meshauth.JwtRawParse(tok)
		t.Log(tokT)
	})

	projectID, _, _ := def.Default.GcpInfo()

	t.Run("K8S GCP federated tokens", func(t *testing.T) {
		sts1 := tokens.NewFederatedTokenSource(&tokens.STSAuthConfig{
			AudienceSource: projectID + ".svc.id.goog",
			//ClusterAddress: fmt.Sprintf("https://container.googleapis.com/v1/projects/%s/locations/%s/clusters/%s",
			//	projectId, clusterLocation, clusterName),

			// Will use TokenRequest to get tokens with AudOverride
			TokenSource: def.Default,
		})
		tok, err := sts1.GetToken(ctx, "http://example.com")
		if err != nil {
			t.Error(err)
		}
		t.Log("Federated access token", tok)

	})

}
