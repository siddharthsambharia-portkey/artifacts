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

// Issue 012: nginx controller renders proxy-auth ConfigMap (with X-Artifact-Proxy-Auth key)
// and wildcard + admin host rules; the proxy-set-headers annotation wires them together.
func TestNginxControllerRendersMiddlewareAndWildcardRouting(t *testing.T) {
	out := helmTemplate(t,
		"--set", "ingress.controller=nginx",
		"--set", "config.auth.mode=header-trust",
		"--set", "headerTrustSecret.secretName=artifact-proxy",
		"--set", "config.domain=artifact.corp.example.com",
	)

	if !strings.Contains(out, "X-Artifact-Proxy-Auth") {
		t.Errorf("expected X-Artifact-Proxy-Auth key in nginx proxy-headers ConfigMap")
	}
	if !strings.Contains(out, "nginx.ingress.kubernetes.io/proxy-set-headers") {
		t.Errorf("expected proxy-set-headers annotation on nginx wildcard Ingress")
	}
	if !strings.Contains(out, "*.artifact.corp.example.com") {
		t.Errorf("expected wildcard host rule *.artifact.corp.example.com in nginx Ingress")
	}
	if !strings.Contains(out, "admin.artifact.corp.example.com") {
		t.Errorf("expected admin host rule admin.artifact.corp.example.com in nginx Ingress")
	}
}

// Issue 012 (regression guard): with ingress.controller unset neither the
// proxy-auth ConfigMap nor the wildcard nginx Ingress renders.
func TestControllerOffRendersNoNginxMiddleware(t *testing.T) {
	out := helmTemplate(t)

	if strings.Contains(out, "nginx-proxy-headers") {
		t.Errorf("nginx-proxy-headers ConfigMap must not render when ingress.controller is unset")
	}
	if strings.Contains(out, "proxy-set-headers") {
		t.Errorf("proxy-set-headers annotation must not render when ingress.controller is unset")
	}
}

// Issue 013: Traefik controller renders proxy-auth Middleware (with X-Artifact-Proxy-Auth)
// and IngressRoute with v3 HostRegexp wildcard + admin host routing.
func TestTraefikControllerRendersMiddlewareAndWildcardRouting(t *testing.T) {
	out := helmTemplate(t,
		"--set", "ingress.controller=traefik",
		"--set", "config.auth.mode=header-trust",
		"--set", "headerTrustSecret.secretName=artifact-proxy",
		"--set", "config.domain=artifact.corp.example.com",
	)

	if !strings.Contains(out, "kind: Middleware") {
		t.Errorf("expected Traefik Middleware resource when ingress.controller=traefik")
	}
	if !strings.Contains(out, "X-Artifact-Proxy-Auth") {
		t.Errorf("expected X-Artifact-Proxy-Auth in Traefik Middleware customRequestHeaders")
	}
	if !strings.Contains(out, "kind: IngressRoute") {
		t.Errorf("expected Traefik IngressRoute when ingress.controller=traefik")
	}
	if !strings.Contains(out, "HostRegexp") {
		t.Errorf("expected HostRegexp wildcard rule in Traefik IngressRoute")
	}
	if !strings.Contains(out, "admin.artifact.corp.example.com") {
		t.Errorf("expected admin host rule in Traefik IngressRoute")
	}
}

// Issue 013 (regression guard): nginx path unchanged — no Traefik resources when controller=nginx.
func TestNginxControllerRendersNoTraefikResources(t *testing.T) {
	out := helmTemplate(t,
		"--set", "ingress.controller=nginx",
		"--set", "config.domain=artifact.corp.example.com",
	)

	if strings.Contains(out, "kind: Middleware") {
		t.Errorf("Traefik Middleware must not render when ingress.controller=nginx")
	}
	if strings.Contains(out, "kind: IngressRoute") {
		t.Errorf("Traefik IngressRoute must not render when ingress.controller=nginx")
	}
}

// Issue 014: storage key fallback mounts GCS credential from Secret and suppresses
// the Workload Identity annotation on the ServiceAccount (mutually exclusive paths).
func TestStorageKeyFallbackMountsGCSKeyAndSuppressesWI(t *testing.T) {
	out := helmTemplate(t,
		"--set", "storageKeyFallback.enabled=true",
		"--set", "storageKeyFallback.secretName=artifact-gcs-key",
		"--set", "config.storage.driver=gcs",
		"--set", `serviceAccount.annotations.iam\.gke\.io/gcp-service-account=artifact@project.iam.gserviceaccount.com`,
	)

	if !strings.Contains(out, "GOOGLE_APPLICATION_CREDENTIALS") {
		t.Errorf("expected GOOGLE_APPLICATION_CREDENTIALS env var when storageKeyFallback enabled for gcs")
	}
	if !strings.Contains(out, "/var/secrets/gcs") {
		t.Errorf("expected GCS key volume mount at /var/secrets/gcs")
	}
	if strings.Contains(out, "iam.gke.io/gcp-service-account") {
		t.Errorf("Workload Identity annotation must not render when storageKeyFallback is enabled")
	}
}

// Issue 014 (regression guard): Workload Identity path renders normally when fallback is off.
func TestWorkloadIdentityPathRendersWhenFallbackOff(t *testing.T) {
	out := helmTemplate(t, "-f", "values-gcp.yaml")

	if strings.Contains(out, "GOOGLE_APPLICATION_CREDENTIALS") {
		t.Errorf("GOOGLE_APPLICATION_CREDENTIALS must not render in the Workload Identity path")
	}
	if !strings.Contains(out, "iam.gke.io/gcp-service-account") {
		t.Errorf("expected Workload Identity annotation on ServiceAccount in the WI path")
	}
}

// Issue 014: for the S3 driver the fallback must source the access/secret keys from a
// Kubernetes Secret reference (secretKeyRef), never as plaintext values.
func TestStorageKeyFallbackS3UsesSecretKeyRefNotPlaintext(t *testing.T) {
	out := helmTemplate(t,
		"--set", "storageKeyFallback.enabled=true",
		"--set", "storageKeyFallback.secretName=artifact-s3-creds",
		"--set", "config.storage.driver=s3",
	)

	if !strings.Contains(out, "ARTIFACT_S3_ACCESS_KEY") {
		t.Errorf("s3 fallback must inject ARTIFACT_S3_ACCESS_KEY")
	}
	if !strings.Contains(out, "ARTIFACT_S3_SECRET_KEY") {
		t.Errorf("s3 fallback must inject ARTIFACT_S3_SECRET_KEY")
	}
	if !strings.Contains(out, "secretKeyRef") {
		t.Errorf("s3 fallback keys must come from a secretKeyRef, not plaintext")
	}
	if !strings.Contains(out, "artifact-s3-creds") {
		t.Errorf("s3 fallback keys must reference the named Kubernetes Secret")
	}
}

// Issue 015: cloudSqlProxy.enabled renders the Cloud SQL Auth Proxy sidecar alongside
// the app container; the sidecar uses the pod's existing identity.
func TestCloudSQLProxySidecarRendersWhenEnabled(t *testing.T) {
	out := helmTemplate(t,
		"--set", "cloudSqlProxy.enabled=true",
		"--set", "cloudSqlProxy.instanceConnectionName=my-project:us-central1:artifact-db",
	)

	if !strings.Contains(out, "name: cloud-sql-proxy") {
		t.Errorf("expected cloud-sql-proxy sidecar container when cloudSqlProxy.enabled=true")
	}
	if !strings.Contains(out, "cloud-sql-connectors/cloud-sql-proxy") {
		t.Errorf("expected Cloud SQL Auth Proxy image in sidecar container")
	}
	if !strings.Contains(out, "my-project:us-central1:artifact-db") {
		t.Errorf("expected instanceConnectionName in sidecar args")
	}
}

// Issue 015 (regression guard): the sidecar is absent by default so the pod
// stays single-container; mirrors TestDefaultProfileStillRendersS3AndOIDC.
func TestCloudSQLProxySidecarAbsentByDefault(t *testing.T) {
	out := helmTemplate(t)

	if strings.Contains(out, "cloud-sql-proxy") {
		t.Errorf("cloud-sql-proxy sidecar must not render unless cloudSqlProxy.enabled=true")
	}
}
