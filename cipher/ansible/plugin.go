//go:build tinygo
// +build tinygo

package main

import (
	"errors"
	"fmt"

	"github.com/extism/go-pdk"
	vault "github.com/sosedoff/ansible-vault-go"
)

const (
	configVaultPassword = "vault_password"
)

func getVaultPassword() (string, error) {
	password, ok := pdk.GetConfig(configVaultPassword)
	if ok {
		return password, nil
	}

	return "", errors.New("no valid vault password was found")

}

//export Encode
func Encode() int32 {
	input := pdk.InputString()
	password, err := getVaultPassword()
	if err != nil {
		pdk.SetError(fmt.Errorf("not valid vault password could be found via explicit key or env."))
		return 1
	}

	str, err := vault.Encrypt(input, password)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	mem := pdk.AllocateString(str)
	pdk.OutputMemory(mem)
	return 0
}

//export Decode
func Decode() int32 {
	input := pdk.InputString()
	password, err := getVaultPassword()
	if err != nil {
		pdk.SetError(fmt.Errorf("not valid vault password could be found via explicit key or env."))
		return 1
	}

	str, err := vault.Decrypt(input, password)
	if err != nil {
		pdk.SetError(err)
		return 1
	}

	mem := pdk.AllocateString(str)
	pdk.OutputMemory(mem)
	return 0
}

func main() {}
