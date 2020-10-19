package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
)

// Amount of reward. Mining the genesis block produed 50 BTC,
// and every 210000 blocks the reward is halved.
const subsidy = 10

// Transaction represents a Bitcoin transaction
type Transaction struct {
	ID   []byte
	Vin  []TXInput
	Vout []TXOutput
}

// IsCoinbase checks whether the transaction is coinbase
func (tx Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

func NewCoinbaseTX(to, data string) *Transaction {
	if data == "" {
		data = fmt.Sprintf("Regard to '%s'", to)
	}

	txin := TXInput{[]byte{}, -1, nil, []byte(data)}
	txout := NewTXOutput(subsidy, to)
	tx := Transaction{nil, []TXInput{txin}, []TXOutput{*txout}}
	tx.ID = tx.Hash()

	return &tx
}

// NewUTXOTransaction creates a new Transaction
func NewUTXOTransaction(from, to string, amount int, UTXOSet *UTXOSet) *Transaction {
	inputs := []TXInput{}
	outputs := []TXOutput{}

	wallets, err := NewWallets()
	if err != nil {
		log.Panic(err)
	}

	wallet := wallets.GetWallet(from)
	pubKeyHash := HashPubKey(wallet.PublicKey)
	acc, validOutputs := UTXOSet.FindSpendableOutputs(pubKeyHash, amount)

	if acc < amount {
		log.Panic("Not enough funds")
	}

	// validOutputs is a map
	// Build a list of inputs
	for txid, outputsIndices := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			log.Panic(err)
		}

		for _, outputIndex := range outputsIndices {
			input := TXInput{txID, outputIndex, nil, wallet.PublicKey}
			inputs = append(inputs, input)
		}
	}

	// Build a list of outputs
	outputs = append(outputs, *NewTXOutput(amount, to))
	if acc >= amount {
		outputs = append(outputs, *NewTXOutput(acc-amount, from))
	}

	tx := Transaction{nil, inputs, outputs}
	tx.ID = tx.Hash()
	UTXOSet.Blockchain.SignTransaction(&tx, wallet.PrivateKey)

	return &tx
}

// Sign each input of a transaction
func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]Transaction) {
	if tx.IsCoinbase() {
		return
	}

	txTrimmed := tx.TrimmedCopy()

	for index, vin := range txTrimmed.Vin {
		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]
		txTrimmed.Vin[index].Signature = nil

		// Pubkey is set to the PubKeyHash of the referenced output
		txTrimmed.Vin[index].PubKey = prevTx.Vout[vin.Vout].PubKeyHash

		// This hash is the data to be signed
		txTrimmed.ID = txTrimmed.Hash()

		// Reset
		txTrimmed.Vin[index].PubKey = nil

		// Sign ID with private key
		r, s, err := ecdsa.Sign(rand.Reader, &privKey, txTrimmed.ID)
		if err != nil {
			log.Panic(err)
		}

		tx.Vin[index].Signature = append(r.Bytes(), s.Bytes()...)
	}
}

// TrimmedCopy creates a trimmed copy of Transaction to be used in signing
func (tx *Transaction) TrimmedCopy() Transaction {
	inputs := []TXInput{}
	outputs := []TXOutput{}

	for _, vin := range tx.Vin {
		// trimmed away signature and pubkey
		inputs = append(inputs, TXInput{vin.Txid, vin.Vout, nil, nil})
	}

	for _, vout := range tx.Vout {
		outputs = append(outputs, TXOutput{vout.Value, vout.PubKeyHash})
	}

	return Transaction{tx.ID, inputs, outputs}
}

// Hash returns the hash of the Transaction
func (tx *Transaction) Hash() []byte {
	hash := [32]byte{}
	txCopy := *tx
	txCopy.ID = []byte{}

	hash = sha256.Sum256(txCopy.Serialize())
	return hash[:]
}

// Serialize transaction
func (tx *Transaction) Serialize() []byte {
	encoded := bytes.Buffer{}

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}

	return encoded.Bytes()
}

func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}

	for _, vin := range tx.Vin {
		if prevTXs[hex.EncodeToString(vin.Txid)].ID == nil {
			log.Panic("Error: previous transaction is not correct")
		}
	}

	txTrimmed := tx.TrimmedCopy()

	// Curve that generates key pairs
	curve := elliptic.P256()

	for index, vin := range tx.Vin {
		prevTx := prevTXs[hex.EncodeToString(vin.Txid)]
		txTrimmed.Vin[index].Signature = nil

		// Pubkey is set to the PubKeyHash of the referenced output
		txTrimmed.Vin[index].PubKey = prevTx.Vout[vin.Vout].PubKeyHash

		// This hash is the data to be signed
		txTrimmed.ID = txTrimmed.Hash()

		// Reset
		txTrimmed.Vin[index].PubKey = nil

		// Signature is a pair of number
		r := big.Int{}
		s := big.Int{}
		sigLen := len(vin.Signature)
		r.SetBytes(vin.Signature[:(sigLen / 2)])
		s.SetBytes(vin.Signature[(sigLen / 2):])

		// Public key is a pair of coordinates
		x := big.Int{}
		y := big.Int{}
		keyLen := len(vin.PubKey)
		x.SetBytes(vin.PubKey[:(keyLen / 2)])
		y.SetBytes(vin.PubKey[(keyLen / 2):])

		rawPubKey := ecdsa.PublicKey{curve, &x, &y}
		if ecdsa.Verify(&rawPubKey, txTrimmed.ID, &r, &s) == false {
			return false
		}
	}

	return true
}
