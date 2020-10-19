package main

import (
	"fmt"
	"log"
)

func (cli *CLI) createBlockchain(address string) {
	if !ValidateAddress(address) {
		log.Panic("ERROR: Address is not valid")
	}
	bc := CreateBlockchain(address)
	defer bc.db.Close()

	// Reindexing happens only right after a new blockchain is created
	UTXOSet := UTXOSet{bc}
	UTXOSet.Reindex()

	fmt.Println("Done!")
}
