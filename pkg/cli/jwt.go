package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type JWTSignCommand struct {
	Issuer                string `help:"The issuer of the JWT token" default:"globalping-hub"`
	Subject               string `help:"The subject of the JWT token" default:"administrator"`
	JWTAuthSecretFromEnv  string `name:"jwt-auth-secret-from-env" help:"Name of the environment variable that contains the JWT secret" default:"JWT_SECRET"`
	JWTAuthSecretFromFile string `name:"jwt-auth-secret-from-file" help:"Path to the file that contains the JWT secret"`
}

func getJWTSecFromSomewhere(envVar string, filePath string) ([]byte, error) {
	if envVar != "" {
		secret := os.Getenv(envVar)
		if secret == "" {
			return nil, fmt.Errorf("JWT secret is not set in environment variable %s", envVar)
		}
		return []byte(secret), nil
	}

	if filePath != "" {
		secret, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to read JWT secret file %s: %v", filePath, err)
		}
		if len(secret) == 0 {
			return nil, fmt.Errorf("JWT secret file %s is empty", filePath)
		}
		return secret, nil
	}

	return nil, fmt.Errorf("no JWT secret is set")
}

func (jwtSignCmd *JWTSignCommand) getJWTSecret() ([]byte, error) {
	return getJWTSecFromSomewhere(jwtSignCmd.JWTAuthSecretFromEnv, jwtSignCmd.JWTAuthSecretFromFile)
}

func (jwtSignCmd *JWTSignCommand) Run() error {

	secret, err := jwtSignCmd.getJWTSecret()
	if err != nil {
		return fmt.Errorf("failed to get JWT secret: %v", err)
	}

	tokenObject := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:   jwtSignCmd.Issuer,
		Subject:  jwtSignCmd.Subject,
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ID:       uuid.New().String(),
	})
	tokenString, err := tokenObject.SignedString(secret)
	if err != nil {
		return fmt.Errorf("failed to sign token: %v", err)
	}
	fmt.Printf("%s\n", tokenString)

	return nil
}

type JWTCommand struct {
	Sign JWTSignCommand `cmd:"sign" help:"Sign a JWT token"`
}
