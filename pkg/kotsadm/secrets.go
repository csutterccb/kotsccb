package kotsadm

import (
	"bytes"
	"os"

	"github.com/google/uuid"
	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/replicatedhq/kots/pkg/crypto"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
)

func getSecretsYAML(deployOptions *DeployOptions) (map[string][]byte, error) {
	docs := map[string][]byte{}
	s := json.NewYAMLSerializer(json.DefaultMetaFactory, scheme.Scheme, scheme.Scheme)

	var jwt bytes.Buffer
	if err := s.Encode(jwtSecret(deployOptions.Namespace, deployOptions.JWT), &jwt); err != nil {
		return nil, errors.Wrap(err, "failed to marshal jwt secret")
	}
	docs["secret-jwt.yaml"] = jwt.Bytes()

	var pg bytes.Buffer
	if err := s.Encode(pgSecret(deployOptions.Namespace, deployOptions.PostgresPassword), &pg); err != nil {
		return nil, errors.Wrap(err, "failed to marshal pg secret")
	}
	docs["secret-pg.yaml"] = pg.Bytes()

	if deployOptions.SharedPasswordBcrypt == "" {
		bcryptPassword, err := bcrypt.GenerateFromPassword([]byte(deployOptions.SharedPassword), 10)
		if err != nil {
			return nil, errors.Wrap(err, "failed to bcrypt shared password")
		}
		deployOptions.SharedPasswordBcrypt = string(bcryptPassword)
	}
	var sharedPassword bytes.Buffer
	if err := s.Encode(sharedPasswordSecret(deployOptions.Namespace, deployOptions.SharedPasswordBcrypt), &sharedPassword); err != nil {
		return nil, errors.Wrap(err, "failed to marshal shared password secret")
	}
	docs["secret-shared-password.yaml"] = sharedPassword.Bytes()

	if deployOptions.APIEncryptionKey == "" {
		cipher, err := crypto.NewAESCypher()
		if err != nil {
			return nil, errors.Wrap(err, "failed to create new API encryption key")
		}
		deployOptions.APIEncryptionKey = cipher.ToString()
	}
	var apiEncryptionBuffer bytes.Buffer
	if err := s.Encode(apiEncryptionKeySecret(deployOptions.Namespace, deployOptions.APIEncryptionKey), &apiEncryptionBuffer); err != nil {
		return nil, errors.Wrap(err, "failed to marshal shared password secret")
	}
	docs["secret-api-encryption.yaml"] = apiEncryptionBuffer.Bytes()

	var s3 bytes.Buffer
	if deployOptions.S3SecretKey == "" {
		deployOptions.S3SecretKey = uuid.New().String()
	}
	if deployOptions.S3AccessKey == "" {
		deployOptions.S3AccessKey = uuid.New().String()
	}
	if err := s.Encode(s3Secret(deployOptions.Namespace, deployOptions.S3AccessKey, deployOptions.S3SecretKey), &s3); err != nil {
		return nil, errors.Wrap(err, "failed to marshal s3 secret")
	}
	docs["secret-s3.yaml"] = s3.Bytes()

	return docs, nil
}

func ensureSecrets(deployOptions *DeployOptions, clientset *kubernetes.Clientset) error {
	if err := ensureJWTSessionSecret(deployOptions.Namespace, clientset); err != nil {
		return errors.Wrap(err, "failed to ensure jwt session secret")
	}

	if err := ensurePostgresSecret(*deployOptions, clientset); err != nil {
		return errors.Wrap(err, "failed to ensure postgres secret")
	}

	if deployOptions.SharedPasswordBcrypt == "" {
		if err := ensureSharedPasswordSecret(deployOptions, clientset); err != nil {
			return errors.Wrap(err, "failed to ensure shared password secret")
		}
	}

	if err := ensureS3Secret(deployOptions.Namespace, clientset); err != nil {
		return errors.Wrap(err, "failed to ensure s3 secret")
	}

	if err := ensureAPIEncryptionSecret(deployOptions, clientset); err != nil {
		return errors.Wrap(err, "failed to ensure s3 secret")
	}

	return nil
}

func getS3Secret(namespace string, clientset *kubernetes.Clientset) (*corev1.Secret, error) {
	s3Secret, err := clientset.CoreV1().Secrets(namespace).Get("kotsadm-minio", metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, errors.Wrap(err, "failed to get s3 secret from cluster")
	}

	return s3Secret, nil
}

func ensureS3Secret(namespace string, clientset *kubernetes.Clientset) error {
	existingS3Secret, err := getS3Secret(namespace, clientset)
	if err != nil {
		return errors.Wrap(err, "failed to check for existing s3 secret")
	}

	if existingS3Secret == nil {
		_, err := clientset.CoreV1().Secrets(namespace).Create(s3Secret(namespace, uuid.New().String(), uuid.New().String()))
		if err != nil {
			return errors.Wrap(err, "failed to create s3 secret")
		}
	}

	return nil
}

func getJWTSessionSecret(namespace string, clientset *kubernetes.Clientset) (*corev1.Secret, error) {
	jwtSecret, err := clientset.CoreV1().Secrets(namespace).Get("kotsadm-session", metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, errors.Wrap(err, "failed to get jwt session secret from cluster")
	}

	return jwtSecret, nil
}

func ensureJWTSessionSecret(namespace string, clientset *kubernetes.Clientset) error {
	existingJWTSessionSecret, err := getJWTSessionSecret(namespace, clientset)
	if err != nil {
		return errors.Wrap(err, "failed to check for existing jwt sesssion secret")
	}

	if existingJWTSessionSecret == nil {
		_, err := clientset.CoreV1().Secrets(namespace).Create(jwtSecret(namespace, uuid.New().String()))
		if err != nil {
			return errors.Wrap(err, "failed to create jwt session secret")
		}
	}

	return nil
}

func getPostgresSecret(namespace string, clientset *kubernetes.Clientset) (*corev1.Secret, error) {
	pgSecret, err := clientset.CoreV1().Secrets(namespace).Get("kotsadm-postgres", metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, errors.Wrap(err, "failed to get postgres secret from cluster")
	}

	return pgSecret, nil
}

func ensurePostgresSecret(deployOptions DeployOptions, clientset *kubernetes.Clientset) error {
	existingPgSecret, err := getPostgresSecret(deployOptions.Namespace, clientset)
	if err != nil {
		return errors.Wrap(err, "failed to check for existing postgres secret")
	}

	if existingPgSecret == nil {
		_, err := clientset.CoreV1().Secrets(deployOptions.Namespace).Create(pgSecret(deployOptions.Namespace, deployOptions.PostgresPassword))
		if err != nil {
			return errors.Wrap(err, "failed to create postgres secret")
		}
	}

	return nil
}

func getSharedPasswordSecret(namespace string, clientset *kubernetes.Clientset) (*corev1.Secret, error) {
	sharedPasswordSecret, err := clientset.CoreV1().Secrets(namespace).Get("kotsadm-password", metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, errors.Wrap(err, "failed to get shared password secret from cluster")
	}

	return sharedPasswordSecret, nil
}

func ensureSharedPasswordSecret(deployOptions *DeployOptions, clientset *kubernetes.Clientset) error {
	if deployOptions.SharedPassword == "" {
		sharedPassword, err := promptForSharedPassword()
		if err != nil {
			return errors.Wrap(err, "failed to prompt for shared password")
		}

		deployOptions.SharedPassword = sharedPassword
	}

	bcryptPassword, err := bcrypt.GenerateFromPassword([]byte(deployOptions.SharedPassword), 10)
	if err != nil {
		return errors.Wrap(err, "failed to bcrypt shared password")
	}

	existingSharedPasswordSecret, err := getSharedPasswordSecret(deployOptions.Namespace, clientset)
	if err != nil {
		return errors.Wrap(err, "failed to check for existing password secret")
	}
	if existingSharedPasswordSecret == nil {
		_, err := clientset.CoreV1().Secrets(deployOptions.Namespace).Create(sharedPasswordSecret(deployOptions.Namespace, string(bcryptPassword)))
		if err != nil {
			return errors.Wrap(err, "failed to create password secret")
		}
	}

	// TODO handle update

	return nil
}

func promptForSharedPassword() (string, error) {
	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . | bold }} ",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | red }} ",
		Success: "{{ . | bold }} ",
	}

	prompt := promptui.Prompt{
		Label:     "Enter a new password to be used for the Admin Console:",
		Templates: templates,
		Mask:      rune('•'),
		Validate: func(input string) error {
			if len(input) < 6 {
				return errors.New("please enter a longer password")
			}

			return nil
		},
	}

	for {
		result, err := prompt.Run()
		if err != nil {
			if err == promptui.ErrInterrupt {
				os.Exit(-1)
			}
			continue
		}

		return result, nil
	}

}

func ensureAPIEncryptionSecret(deployOptions *DeployOptions, clientset *kubernetes.Clientset) error {
	secret, err := getAPIEncryptionSecret(deployOptions.Namespace, clientset)
	if err != nil {
		return errors.Wrap(err, "failed to check for existing postgres secret")
	}

	if secret != nil {
		if key, _ := secret.Data["encryptionKey"]; len(key) > 0 {
			return nil
		}
	}

	if deployOptions.APIEncryptionKey == "" {
		cipher, err := crypto.NewAESCypher()
		if err != nil {
			return errors.Wrap(err, "failed to create new AES cipher")
		}
		deployOptions.APIEncryptionKey = cipher.ToString()
	}

	_, err = clientset.CoreV1().Secrets(deployOptions.Namespace).Create(apiEncryptionKeySecret(deployOptions.Namespace, deployOptions.APIEncryptionKey))
	if err != nil {
		return errors.Wrap(err, "failed to create API encryption secret")
	}

	return nil
}

func getAPIEncryptionSecret(namespace string, clientset *kubernetes.Clientset) (*corev1.Secret, error) {
	apiSecret, err := clientset.CoreV1().Secrets(namespace).Get("kotsadm-encryption", metav1.GetOptions{})
	if err != nil {
		if kuberneteserrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, errors.Wrap(err, "failed to get postgres secret from cluster")
	}

	return apiSecret, nil
}
