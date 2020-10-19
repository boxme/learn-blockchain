package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"
)

type Block struct {
	Timestamp     int64
	Transactions  []*Transaction
	PrevBlockHash []byte
	Hash          []byte
	Nonce         int
}

func NewBlock(transaction []*Transaction, prevBlockHash []byte) *Block {
	block := &Block{time.Now().Unix(), transaction, prevBlockHash, []byte{}, 0}
	pow := NewProofOfWork(block)
	nonce, hash := pow.Run()

	block.Hash = hash[:]
	block.Nonce = nonce

	return block
}

func NewGenesisBlock(coinbase *Transaction) *Block {
	return NewBlock([]*Transaction{coinbase}, []byte{})
}

func (b *Block) Serialize() []byte {
	result := &bytes.Buffer{}
	encoder := gob.NewEncoder(result)

	err := encoder.Encode(b)
	if err != nil {
		log.Panic(err)
	}

	return result.Bytes()
}

func DeserializeBlock(d []byte) *Block {
	block := &Block{}
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(block)
	if err != nil {
		log.Panic(err)
	}

	return block
}

// HashTransactions returns a hash of the transactions in the block
func (b *Block) HashTransactions() []byte {
	transactions := [][]byte{}

	for _, tx := range b.Transactions {
		transactions = append(transactions, tx.Serialize())
	}

	tree := NewMerkleTree(transactions)

	return tree.RootNode.Data
}
