package main

import (
	"encoding/hex"
	"log"

	"github.com/boltdb/bolt"
)

const utxoBucket = "chainstate"

// UTXOSet represents unspent transaction outputs
type UTXOSet struct {
	Blockchain *Blockchain
}

// FindSpendableOutputs finds and returns unspent outputs to reference in inputs
func (u UTXOSet) FindSpendableOutputs(pubkeyHash []byte, amount int) (int, map[string][]int) {
	unspentOutputs := make(map[string][]int)
	accumulated := 0
	db := u.Blockchain.db

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		cursor := b.Cursor()

		for key, value := cursor.First(); key != nil; key, value = cursor.Next() {
			txID := hex.EncodeToString(key)
			outputs := DeserializeOutputs(value)

			for outputIndex, output := range outputs.Outputs {
				if output.IsLockedWithKey(pubkeyHash) && accumulated < amount {
					accumulated += output.Value
					unspentOutputs[txID] = append(unspentOutputs[txID], outputIndex)
				}
			}
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	return accumulated, unspentOutputs
}

// Reindex rebuilds the UTXO set
func (u UTXOSet) Reindex() {
	db := u.Blockchain.db
	bucketName := []byte(utxoBucket)

	err := db.Update(func(tx *bolt.Tx) error {
		err := tx.DeleteBucket(bucketName)
		if err != nil && err != bolt.ErrBucketNotFound {
			log.Panic(err)
		}

		_, err = tx.CreateBucket(bucketName)
		if err != nil {
			log.Panic(err)
		}

		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	UTXO := u.Blockchain.FindUTXO()

	err = db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)

		for txID, outputs := range UTXO {
			key, err := hex.DecodeString(txID)
			if err != nil {
				log.Panic(err)
			}

			err = b.Put(key, outputs.Serialize())
			if err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
}

// FindUTXO returns all unspent transaction outputs
func (u UTXOSet) FindUTXO(pubKeyHash []byte) []TXOutput {
	UTXOs := []TXOutput{}
	db := u.Blockchain.db

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))
		c := b.Cursor()

		for key, value := c.First(); key != nil; key, value = c.Next() {
			outputs := DeserializeOutputs(value)

			for _, output := range outputs.Outputs {
				if output.IsLockedWithKey(pubKeyHash) {
					UTXOs = append(UTXOs, output)
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
	return UTXOs
}

// Update updates the UTXO set with transactions from the Block
// The Block is considered to be the tip of a blockchain
func (u UTXOSet) Update(block *Block) {
	db := u.Blockchain.db

	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(utxoBucket))

		for _, tx := range block.Transactions {
			if tx.IsCoinbase() == false {
				for _, vin := range tx.Vin {
					updatedOutputs := TXOutputs{}
					outputsBytes := b.Get(vin.Txid)
					outputs := DeserializeOutputs(outputsBytes)

					for outputIndex, output := range outputs.Outputs {
						if outputIndex != vin.Vout {
							// Unspent outputs
							updatedOutputs.Outputs = append(updatedOutputs.Outputs, output)
						}
					}

					if len(updatedOutputs.Outputs) == 0 {
						err := b.Delete(vin.Txid)
						if err != nil {
							log.Panic(err)
						}
					} else {
						err := b.Put(vin.Txid, updatedOutputs.Serialize())
						if err != nil {
							log.Panic(err)
						}
					}
				}
			}

			// Outputs at the tip of the chain
			newOutputs := TXOutputs{}
			for _, output := range tx.Vout {
				newOutputs.Outputs = append(newOutputs.Outputs, output)
			}

			err := b.Put(tx.ID, newOutputs.Serialize())
			if err != nil {
				log.Panic(err)
			}
		}
		return nil
	})

	if err != nil {
		log.Panic(err)
	}
}
