package helm

import (
	"os/exec"
	"strings"
	"testing"
)

// helmTemplate renders the chart in this directory with the given extra args
// (e.g. value files and --set overrides) and returns the combined manifest
// stream. It skips when the helm binary is unavailable so the suite stays
// runnable in environments without helm installed.
func helmTemplate(t *testing.T, args ...string) string {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm binary not found; skipping chart render test")
	}
	cmd := exec.Command("helm", append([]string{"template", "artifact", "."}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("helm template failed: %v\n%s", err, out)
	}
	return string(out)
}

// Issue 7: the GCP profile must render the gcs driver and a Workload Identity
// ServiceAccount, and must NOT inject any S3 access keys.
func TestGCSProfileRendersWorkloadIdentityWithoutS3Keys(t *testing.T) {
	out := helmTemplate(t, "-f", "values-gcp.yaml")

	if !strings.Contains(out, `driver: "gcs"`) {
		t.Errorf("expected gcs storage driver in render")
	}
	if !strings.Contains(out, "iam.gke.io/gcp-service-account") {
		t.Errorf("expected Workload Identity annotation on the ServiceAccount")
	}
	if strings.Contains(out, "ARTIFACT_S3_ACCESS_KEY") {
		t.Errorf("gcs profile must not inject S3 access-key env vars")
	}
}

// Issue 7: header-trust auth must render the header_trust config block and
// inject the proxy shared secret from a Kubernetes Secret (never plaintext).
func TestHeaderTrustInjectsProxySecretFromSecret(t *testing.T) {
	out := helmTemplate(t,
		"--set", "config.auth.mode=header-trust",
		"--set", "headerTrustSecret.secretName=artifact-proxy",
	)

	if !strings.Contains(out, "header_trust:") {
		t.Errorf("expected header_trust config block in render")
	}
	if !strings.Contains(out, "name: ARTIFACT_PROXY_SECRET") {
		t.Errorf("expected ARTIFACT_PROXY_SECRET env var in render")
	}
	if !strings.Contains(out, "secretKeyRef") || !strings.Contains(out, "artifact-proxy") {
		t.Errorf("proxy secret must be sourced from a Kubernetes Secret, got:\n%s", out)
	}
}

// Issue 7 (regression guard): the default values still render the existing
// S3 + native-OIDC path so the GCS work does not change the baseline.
func TestDefaultProfileStillRendersS3AndOIDC(t *testing.T) {
	out := helmTemplate(t)

	if !strings.Contains(out, `driver: "s3"`) {
		t.Errorf("default profile should still use the s3 storage driver")
	}
	if !strings.Contains(out, `mode: "oidc"`) {
		t.Errorf("default profile should still use native oidc auth")
	}
	if !strings.Contains(out, "ARTIFACT_S3_ACCESS_KEY") {
		t.Errorf("default profile should still inject S3 access-key env vars")
	}
}

// Issue 8: with certificate.enabled the chart renders a cert-manager
// Certificate covering both the wildcard and the apex host.
func TestWildcardCertificateRendersWhenEnabled(t *testing.T) {
	out := helmTemplate(t,
		"--set", "certificate.enabled=true",
		"--set", "certificate.clusterIssuer=letsencrypt-clouddns",
		"--set", "config.domain=artifact.corp.example.com",
	)

	if !strings.Contains(out, "kind: Certificate") {
		t.Fatalf("expected a cert-manager Certificate resource when enabled")
	}
	if !strings.Contains(out, `"*.artifact.corp.example.com"`) {
		t.Errorf("Certificate must cover the wildcard host")
	}
	if !strings.Contains(out, `"artifact.corp.example.com"`) {
		t.Errorf("Certificate must cover the apex host")
	}
	if !strings.Contains(out, "letsencrypt-clouddns") {
		t.Errorf("Certificate must reference the configured ClusterIssuer")
	}
}

// Issue 8: the Certificate is flag-gated and must be absent by default so
// clusters without cert-manager CRDs are not broken.
func TestWildcardCertificateAbsentByDefault(t *testing.T) {
	out := helmTemplate(t)

	if strings.Contains(out, "kind: Certificate") {
		t.Errorf("Certificate should not render unless certificate.enabled=true")
	}
}
