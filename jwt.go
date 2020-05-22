package main

import (
	"crypto/ed25519"
	"github.com/pascaldekloe/jwt"

	"fmt"
	"time"
)

var JWTPrivateKey ed25519.PrivateKey
var JWTPublicKey ed25519.PublicKey

func init() {
	JWTPrivateKey = ed25519.NewKeyFromSeed([]byte("eyJhbGciOiJFUzI1NiJ9OiJFUzI1NiJ9"))
	JWTPublicKey = []byte(JWTPrivateKey)[32:]
}

func createUserToken(id int) ([]byte, error) {
	var claims jwt.Claims
	claims.Subject = "user"
	claims.Issued = jwt.NewNumericTime(time.Now().Round(time.Second))
	claims.Set = map[string]interface{}{"id": id}
	return claims.EdDSASign(JWTPrivateKey)
}

func verifyUserToken(token []byte) (int, error) {
	claims, err := jwt.EdDSACheck(token, JWTPublicKey)
	if err != nil {
		return 0, err
	}
	if !claims.Valid(time.Now()) {
		return 0, fmt.Errorf("credential time constraints exceeded")
	}
	if claims.Subject != "user" {
		return 0, fmt.Errorf("wrong claims subject")
	}

	id, _ := claims.Set["id"].(float64)
	return int(id), nil
}
