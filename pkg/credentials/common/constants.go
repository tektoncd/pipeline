package common

// Secret key constants used in credential files,
// so as to avoid reliance on corev1.Secret.
//
//nolint:gosec // for known Kubernetes secret-type constants, not real credentials
const (
	BasicAuthUsernameKey          = "username"
	BasicAuthPasswordKey          = "password"
	SSHAuthPrivateKey             = "ssh-privatekey"
	DockerConfigKey               = ".dockercfg"
	DockerConfigJsonKey           = ".dockerconfigjson"
	SecretTypeBasicAuth           = "kubernetes.io/basic-auth"
	SecretTypeSSHAuth             = "kubernetes.io/ssh-auth"
	SecretTypeDockerConfigJson    = "kubernetes.io/dockerconfigjson"
	SecretTypeDockercfg           = "kubernetes.io/dockercfg"
	SecretTypeServiceAccountToken = "kubernetes.io/service-account-token"
	SecretTypeOpaque              = "kubernetes.io/opaque"
	SecretTypeTLS                 = "kubernetes.io/tls"
	SecretTypeBootstrapToken      = "kubernetes.io/bootstrap-token"
)
