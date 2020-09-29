package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
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

func (tx *Transaction) SetID() {
	encoded := bytes.Buffer{}
	hash := [32]byte{}

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}
	hash = sha256.Sum256(encoded.Bytes())
	tx.ID = hash[:]
}

// Checks whetherthe transaction is coinbase
func (tx Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

// TXInput represents a transaction input
type TXInput struct {
	Txid      []byte
	Vout      int // Index of an output in the transaction
	ScriptSig string
}

// Checks whether the address initiated the transaction
func (in *TXInput) CanUnlockOutputWith(unlockingData string) bool {
	return in.ScriptSig == unlockingData
}

// TXOutput represents a transaction output
type TXOutput struct {
	Value        int
	ScriptPubKey string
}

// Checks if the output can be unlocked with the provided data
func (out *TXOutput) CanBeUnlockedWith(unlockingData string) bool {
	return out.ScriptPubKey == unlockingData
}

func NewCoinbaseTX(to, data string) *Transaction {
	if data == "" {
		data = fmt.Sprintf("Regard to '%s'", to)
	}

	txin := TXInput{[]byte{}, -1, data}
	txout := TXOutput{subsidy, to}
	tx := Transaction{nil, []TXInput{txin}, []TXOutput{txout}}
	tx.SetID()

	return &tx
}

// Creates a new Transaction
func NewUTXOTransaction(from, to string, amount int, bc *Blockchain) *Transaction {
	inputs := []TXInput{}
	outputs := []TXOutput{}

	acc, validOutputs := bc.FindSpendableOutputs(from, amount)

	if acc < amount {
		log.Panic("Not enough funds")
	}

	// Build a list of inputs
	for txid, outputs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		if err != nil {
			log.Panic(err)
		}

		for _, output := range outputs {
			input := TXInput{txID, output, from}
			inputs = append(inputs, input)
		}
	}

	// Build a list of outputs
	outputs = append(outputs, TXOutput{amount, to})
	if acc >= amount {
		outputs = append(outputs, TXOutput{acc - amount, from})
	}

	tx := Transaction{nil, inputs, outputs}
	tx.SetID()

	return &tx
}
