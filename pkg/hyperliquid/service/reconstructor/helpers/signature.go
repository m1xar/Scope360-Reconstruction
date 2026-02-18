package helpers

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func CreateWalletAndSign(message string) (address string, signatureHex string, err error) { //тестовый метод на создание волета и подпись меседжа, интернал вызовы убрал, но можно юзать для теста
	privKey, err := crypto.GenerateKey()
	if err != nil {
		return "", "", err
	}

	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	msgHash := crypto.Keccak256Hash([]byte(msg))

	sig, err := crypto.Sign(msgHash.Bytes(), privKey)
	if err != nil {
		return "", "", err
	}

	address = crypto.PubkeyToAddress(privKey.PublicKey).Hex()
	signatureHex = "0x" + common.Bytes2Hex(sig)
	return address, signatureHex, nil
}

func VerifySignature(expectedAddress, signatureHex, message string) bool {
	msg := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	msgHash := crypto.Keccak256Hash([]byte(msg))

	sig := common.FromHex(signatureHex)
	if len(sig) != 65 {
		return false
	}
	if sig[64] >= 27 {
		sig[64] -= 27
	}
	if sig[64] != 0 && sig[64] != 1 {
		return false
	}

	pubKey, err := crypto.SigToPub(msgHash.Bytes(), sig)
	if err != nil {
		return false
	}

	recovered := crypto.PubkeyToAddress(*pubKey).Hex()
	return strings.EqualFold(recovered, expectedAddress)
}
