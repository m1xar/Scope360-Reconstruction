package orderly

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	ChainTypeEVM = "EVM"
	ChainTypeSOL = "SOL"
)

type Credentials struct {
	AccountID  string
	PublicKey  string
	PrivateKey ed25519.PrivateKey
	ChainType  string
}

func NewCredentials(walletAddress, brokerID, ed25519PubKey string, ed25519PrivKey ed25519.PrivateKey) Credentials {
	chainType := DetectChainType(walletAddress)
	return Credentials{
		AccountID:  ComputeAccountID(walletAddress, brokerID),
		PublicKey:  ed25519PubKey,
		PrivateKey: ed25519PrivKey,
		ChainType:  chainType,
	}
}

func DetectChainType(walletAddress string) string {
	if strings.HasPrefix(walletAddress, "0x") || strings.HasPrefix(walletAddress, "0X") {
		return ChainTypeEVM
	}
	return ChainTypeSOL
}

func ComputeAccountID(walletAddress, brokerID string) string {
	brokerHash := crypto.Keccak256([]byte(brokerID))
	chainType := DetectChainType(walletAddress)

	encoded := make([]byte, 64)

	if chainType == ChainTypeSOL {

		solPubKey, err := base58Decode(walletAddress)
		if err != nil || len(solPubKey) != 32 {

			padded := make([]byte, 32)
			copy(padded, solPubKey)
			solPubKey = padded
		}
		copy(encoded[0:32], solPubKey)
	} else {

		addressBytes := common.HexToAddress(walletAddress).Bytes()
		copy(encoded[12:32], addressBytes)
	}

	copy(encoded[32:64], brokerHash)

	accountHash := crypto.Keccak256(encoded)
	return "0x" + common.Bytes2Hex(accountHash)
}

func SignRequest(privateKey ed25519.PrivateKey, timestamp int64, method, path, body string) string {
	message := strconv.FormatInt(timestamp, 10) + method + path + body
	sig := ed25519.Sign(privateKey, []byte(message))
	return base64.URLEncoding.EncodeToString(sig)
}

func BuildAuthHeaders(creds Credentials, method, path, body string) map[string]string {
	ts := time.Now().UnixMilli()
	sig := SignRequest(creds.PrivateKey, ts, method, path, body)

	pubKey := creds.PublicKey
	if !strings.HasPrefix(pubKey, "ed25519:") {
		pubKey = "ed25519:" + pubKey
	}

	return map[string]string{
		"orderly-account-id": creds.AccountID,
		"orderly-key":        pubKey,
		"orderly-timestamp":  strconv.FormatInt(ts, 10),
		"orderly-signature":  sig,
	}
}

func VerifyWalletSignature(expectedAddress, signatureHex, message string) bool {
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

func ParseEd25519PrivateKey(seed string) (ed25519.PrivateKey, error) {
	decoded, err := base58Decode(seed)
	if err != nil {
		return nil, fmt.Errorf("invalid base58 seed: %w", err)
	}
	if len(decoded) != ed25519.SeedSize {
		return nil, fmt.Errorf("seed must be %d bytes, got %d", ed25519.SeedSize, len(decoded))
	}
	return ed25519.NewKeyFromSeed(decoded), nil
}

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func base58Decode(input string) ([]byte, error) {
	result := big.NewInt(0)
	base := big.NewInt(58)

	for _, c := range input {
		idx := strings.IndexRune(base58Alphabet, c)
		if idx < 0 {
			return nil, fmt.Errorf("invalid base58 character: %c", c)
		}
		result.Mul(result, base)
		result.Add(result, big.NewInt(int64(idx)))
	}

	decoded := result.Bytes()

	leadingZeros := 0
	for _, c := range input {
		if c == '1' {
			leadingZeros++
		} else {
			break
		}
	}

	if leadingZeros > 0 {
		decoded = append(make([]byte, leadingZeros), decoded...)
	}

	return decoded, nil
}
