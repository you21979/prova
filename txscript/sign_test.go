// Copyright (c) 2013-2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package txscript_test

import (
	"errors"
	"fmt"
	"github.com/bitgo/rmgd/blockchain"
	"github.com/bitgo/rmgd/btcec"
	"github.com/bitgo/rmgd/chaincfg"
	"github.com/bitgo/rmgd/chaincfg/chainhash"
	"github.com/bitgo/rmgd/rmgutil"
	"github.com/bitgo/rmgd/txscript"
	"github.com/bitgo/rmgd/wire"
	"testing"
)

type addressToKey struct {
	key        *btcec.PrivateKey
	compressed bool
}

func mkGetKey(keys map[string]addressToKey) txscript.KeyDB {
	if keys == nil {
		return txscript.KeyClosure(func(addr rmgutil.Address) (*btcec.PrivateKey,
			bool, error) {
			return nil, false, errors.New("nope")
		})
	}
	return txscript.KeyClosure(func(addr rmgutil.Address) (*btcec.PrivateKey,
		bool, error) {
		a2k, ok := keys[addr.EncodeAddress()]
		if !ok {
			return nil, false, errors.New("nope")
		}
		return a2k.key, a2k.compressed, nil
	})
}

func mkGetScript(scripts map[string][]byte) txscript.ScriptDB {
	if scripts == nil {
		return txscript.ScriptClosure(func(addr rmgutil.Address) (
			[]byte, error) {
			return nil, errors.New("nope")
		})
	}
	return txscript.ScriptClosure(func(addr rmgutil.Address) ([]byte,
		error) {
		script, ok := scripts[addr.EncodeAddress()]
		if !ok {
			return nil, errors.New("nope")
		}
		return script, nil
	})
}

func mkGetHash(hashes map[string][]rmgutil.Address) txscript.HashDB {
	if hashes == nil {
		return txscript.HashClosure(func(addr rmgutil.Address) (
			[]rmgutil.Address, error) {
			return nil, errors.New("nope")
		})
	}
	return txscript.HashClosure(func(addr rmgutil.Address) ([]rmgutil.Address,
		error) {
		hash, ok := hashes[addr.EncodeAddress()]
		if !ok {
			return nil, errors.New("nope")
		}
		return hash, nil
	})
}

func checkScripts(msg string, tx *wire.MsgTx, idx int, inputAmt int64, sigScript []byte, pkScript []byte) error {
	tx.TxIn[idx].SignatureScript = sigScript

	// Before passing the script to the VM, we check whether it is an Aztec script.
	pops, err := txscript.ParseScript(pkScript)
	if err != nil {
		return fmt.Errorf("failed to parse script %s: %v", msg, err)
	}
	// If script is Aztec script, we replace all keyIDs with pubKeyHashes.
	if txscript.TypeOfScript(pops) == txscript.AztecTy {
		//
		// Uncomment this to print out raw transaction.
		// Edit data in TestSignTxOutput to construct the tx you want.
		//
		// var testTxBuf bytes.Buffer
		// tx.Serialize(&testTxBuf)
		// fmt.Printf("tx %v: %x \n", idx, testTxBuf.Bytes())
		// fmt.Printf("pk %v: %x \n", idx, pkScript)
		keyIDs, err := txscript.ExtractKeyIDs(pops)
		view := blockchain.NewUtxoViewpoint()
		keyIdMap := view.LookupKeyIDs(keyIDs)
		txscript.ReplaceKeyIDs(pops, keyIdMap)
		pkScript, err = txscript.UnparseScript(pops)
		if err != nil {
			return err
		}
	}

	vm, err := txscript.NewEngine(pkScript, tx, idx,
		txscript.ScriptBip16|txscript.ScriptVerifyDERSignatures, nil, nil, inputAmt)
	if err != nil {
		return fmt.Errorf("failed to make script engine for %s: %v",
			msg, err)
	}

	err = vm.Execute()
	if err != nil {
		return fmt.Errorf("invalid script signature for %s: %v", msg,
			err)
	}

	return nil
}

func signAndCheck(msg string, tx *wire.MsgTx, idx int, inputAmt int64, pkScript []byte,
	hashType txscript.SigHashType, kdb txscript.KeyDB, sdb txscript.ScriptDB, hdb txscript.HashDB,
	previousScript []byte) error {
	sigScript, err := txscript.SignTxOutput(&chaincfg.TestNet3Params, tx,
		idx, inputAmt, pkScript, hashType, kdb, sdb, hdb, nil)
	if err != nil {
		return fmt.Errorf("failed to sign output %s: %v", msg, err)
	}

	return checkScripts(msg, tx, idx, inputAmt, sigScript, pkScript)
}

func TestSignTxOutput(t *testing.T) {
	t.Parallel()

	// make key
	// make script based on key.
	// sign with magic pixie dust.
	hashTypes := []txscript.SigHashType{
		txscript.SigHashOld, // no longer used but should act like all
		txscript.SigHashAll,
		txscript.SigHashNone,
		txscript.SigHashSingle,
		txscript.SigHashAll | txscript.SigHashAnyOneCanPay,
		txscript.SigHashNone | txscript.SigHashAnyOneCanPay,
		txscript.SigHashSingle | txscript.SigHashAnyOneCanPay,
	}
	inputAmounts := []int64{5000000000, 5000000000, 5000000000}
	hash01, _ := chainhash.NewHashFromStr("08886fe11cc704bc617ebaf50f8bed16a66da84141d26d786a054f2c361c905a")
	hash02, _ := chainhash.NewHashFromStr("7fd6b408c31e2e1551c6285d9d3249e6263b6f2bc33c30d61a75f240caba902e")
	hash03, _ := chainhash.NewHashFromStr("9420d59e07d26a39eebadd79dea7f7e2f72d2fe0203fc8fb9a9e08f43fa9731e")
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []*wire.TxIn{
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  *hash01,
					Index: 0,
				},
				Sequence: 4294967295,
			},
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  *hash02,
					Index: 0,
				},
				Sequence: 4294967295,
			},
			{
				PreviousOutPoint: wire.OutPoint{
					Hash:  *hash03,
					Index: 0,
				},
				Sequence: 4294967295,
			},
		},
		TxOut: []*wire.TxOut{
			{
				Value: int64(1000000000),
			},
			{
				Value: int64(2000000000),
			},
			{
				Value: int64(11900000000),
			},
		},
		LockTime: 0,
	}

	// Pay to Pubkey Hash (uncompressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)
			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := rmgutil.NewAddressPubKeyHash(
				rmgutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), nil, nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to Pubkey Hash (uncompressed) (merging with correct)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)
			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := rmgutil.NewAddressPubKeyHash(
				rmgutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			sigScript, err := txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], pkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err = txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], pkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), nil, sigScript)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, pkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Pay to Pubkey Hash (compressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := rmgutil.NewAddressPubKeyHash(
				rmgutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(nil), nil, nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to Pubkey Hash (compressed) with duplicate merge
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := rmgutil.NewAddressPubKeyHash(
				rmgutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			sigScript, err := txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], pkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(nil), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err = txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], pkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(nil), nil, sigScript)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, pkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Pay to PubKey (uncompressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := rmgutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), nil, nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to PubKey (uncompressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := rmgutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			sigScript, err := txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], pkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err = txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], pkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(nil), nil, sigScript)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, pkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Pay to PubKey (compressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := rmgutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i], pkScript, hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(nil), nil, nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to PubKey (compressed) with duplicate merge
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := rmgutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			sigScript, err := txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], pkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(nil), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err = txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], pkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(nil), nil, sigScript)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, pkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// As before, but with p2sh now.
	// Pay to Pubkey Hash (uncompressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)
			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := rmgutil.NewAddressPubKeyHash(
				rmgutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
				break
			}

			scriptAddr, err := rmgutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := txscript.PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i], scriptPkScript,
				hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to Pubkey Hash (uncompressed) with duplicate merge
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)
			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := rmgutil.NewAddressPubKeyHash(
				rmgutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
				break
			}

			scriptAddr, err := rmgutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := txscript.PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			sigScript, err := txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], scriptPkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err = txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], scriptPkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, scriptPkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Pay to Pubkey Hash (compressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := rmgutil.NewAddressPubKeyHash(
				rmgutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := rmgutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := txscript.PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i], scriptPkScript,
				hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to Pubkey Hash (compressed) with duplicate merge
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := rmgutil.NewAddressPubKeyHash(
				rmgutil.Hash160(pk), &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := rmgutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := txscript.PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			sigScript, err := txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], scriptPkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err = txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], scriptPkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, scriptPkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Pay to PubKey (uncompressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := rmgutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := rmgutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := txscript.PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i], scriptPkScript,
				hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to PubKey (uncompressed) with duplicate merge
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeUncompressed()
			address, err := rmgutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := rmgutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := txscript.PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			sigScript, err := txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], scriptPkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err = txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], scriptPkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, false},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, scriptPkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Pay to PubKey (compressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := rmgutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := rmgutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := txscript.PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i], scriptPkScript,
				hashType,
				mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	// Pay to PubKey (compressed)
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk := (*btcec.PublicKey)(&key.PublicKey).
				SerializeCompressed()
			address, err := rmgutil.NewAddressPubKey(pk,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.PayToAddrScript(address)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := rmgutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := txscript.PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			sigScript, err := txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], scriptPkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// by the above loop, this should be valid, now sign
			// again and merge.
			sigScript, err = txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], scriptPkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address.EncodeAddress(): {key, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s a "+
					"second time: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript, scriptPkScript)
			if err != nil {
				t.Errorf("twice signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Basic Multisig
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key1, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk1 := (*btcec.PublicKey)(&key1.PublicKey).
				SerializeCompressed()
			address1, err := rmgutil.NewAddressPubKey(pk1,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			key2, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey 2 for %s: %v",
					msg, err)
				break
			}

			pk2 := (*btcec.PublicKey)(&key2.PublicKey).
				SerializeCompressed()
			address2, err := rmgutil.NewAddressPubKey(pk2,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address 2 for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.MultiSigScript(
				[]*rmgutil.AddressPubKey{address1, address2},
				2)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := rmgutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := txscript.PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			if err := signAndCheck(msg, tx, i, inputAmounts[i], scriptPkScript,
				hashType,
				mkGetKey(map[string]addressToKey{
					address1.EncodeAddress(): {key1, true},
					address2.EncodeAddress(): {key2, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil); err != nil {
				t.Error(err)
				break
			}
		}
	}

	//Aztec Multisig
	hashType := txscript.SigHashAll
	for i := range tx.TxIn {
		msg := fmt.Sprintf("%d:%d", hashType, i)

		//KeyID #1
		keyId1 := rmgutil.KeyIDFromAddressBuffer([]byte{0, 0, 0, 1})
		key1, pk1 := btcec.PrivKeyFromBytes(btcec.S256(), []byte{
			0xea, 0xf0, 0x2c, 0xa3, 0x48, 0xc5, 0x24, 0xe6,
			0x39, 0x26, 0x55, 0xba, 0x4d, 0x29, 0x60, 0x3c,
			0xd1, 0xa7, 0x34, 0x7d, 0x9d, 0x65, 0xcf, 0xe9,
			0x3c, 0xe1, 0xeb, 0xff, 0xdc, 0xa2, 0x26, 0x94,
		})
		//The following hash is hardcoded in opcode.go, and will replace keyId  0x00000001
		//pk1 hash [53 219 191 4 188 160 97 228 157 172 224 143 133 141 135 117 192 165 124 142]
		addr1, err := rmgutil.NewAddressPubKey(pk1.SerializeCompressed(),
			&chaincfg.TestNet3Params)
		if err != nil {
			t.Errorf("failed to make address for %s: %v",
				msg, err)
			break
		}

		//KeyID #2
		keyId2 := rmgutil.KeyIDFromAddressBuffer([]byte{0, 0, 0, 2})
		key2, pk2 := btcec.PrivKeyFromBytes(btcec.S256(), []byte{
			0x2b, 0x8c, 0x52, 0xb7, 0x7b, 0x32, 0x7c, 0x75,
			0x5b, 0x9b, 0x37, 0x55, 0x00, 0xd3, 0xf4, 0xb2,
			0xda, 0x9b, 0x0a, 0x1f, 0xf6, 0x5f, 0x68, 0x91,
			0xd3, 0x11, 0xfe, 0x94, 0x29, 0x5b, 0xc2, 0x6a,
		})
		//The following hash is hardcoded in opcode.go, and will replace keyId  0x00000002
		//pk2 hash [207 85 250 254 141 22 106 190 101 133 28 207 125 127 53 172 186 5 176 249]
		addr2, err := rmgutil.NewAddressPubKey(pk2.SerializeCompressed(),
			&chaincfg.TestNet3Params)
		if err != nil {
			t.Errorf("failed to make address for %s: %v",
				msg, err)
			break
		}

		//dynamic key definition as usual
		key3, err := btcec.NewPrivateKey(btcec.S256())
		if err != nil {
			t.Errorf("failed to make privKey for %s: %v",
				msg, err)
			break
		}
		pk3 := (*btcec.PublicKey)(&key3.PublicKey)
		addr3, err := rmgutil.NewAddressPubKey(pk3.SerializeCompressed(),
			&chaincfg.TestNet3Params)
		if err != nil {
			t.Errorf("failed to make address for %s: %v",
				msg, err)
			break
		}
		pkHash := rmgutil.Hash160(pk3.SerializeCompressed())

		//Creation of Aztec address
		addr, err := rmgutil.NewAddressAztec(pkHash,
			[]rmgutil.KeyID{keyId1, keyId2}, &chaincfg.TestNet3Params)
		if err != nil {
			t.Errorf("failed to make Aztec address for %s: %v",
				msg, err)
			break
		}

		scriptPkScript, err := txscript.PayToAddrScript(
			addr)
		if err != nil {
			t.Errorf("failed to make script pkscript for "+
				"%s: %v", msg, err)
			break
		}

		if err := signAndCheck(msg, tx, i, inputAmounts[i], scriptPkScript,
			hashType,
			mkGetKey(map[string]addressToKey{
				addr1.EncodeAddress(): {key1, true},
				addr2.EncodeAddress(): {key2, true},
				addr3.EncodeAddress(): {key3, true},
			}), nil, mkGetHash(map[string][]rmgutil.Address{
				addr.EncodeAddress(): []rmgutil.Address{addr1, addr2, addr3},
			}), nil); err != nil {
			t.Error(err)
			break
		}
	}

	// Two part multisig, sign with one key then the other.
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key1, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk1 := (*btcec.PublicKey)(&key1.PublicKey).
				SerializeCompressed()
			address1, err := rmgutil.NewAddressPubKey(pk1,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			key2, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey 2 for %s: %v",
					msg, err)
				break
			}

			pk2 := (*btcec.PublicKey)(&key2.PublicKey).
				SerializeCompressed()
			address2, err := rmgutil.NewAddressPubKey(pk2,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address 2 for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.MultiSigScript(
				[]*rmgutil.AddressPubKey{address1, address2},
				2)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := rmgutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := txscript.PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			sigScript, err := txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], scriptPkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address1.EncodeAddress(): {key1, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// Only 1 out of 2 signed, this *should* fail.
			if checkScripts(msg, tx, i, inputAmounts[i], sigScript,
				scriptPkScript) == nil {
				t.Errorf("part signed script valid for %s", msg)
				break
			}

			// Sign with the other key and merge
			sigScript, err = txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], scriptPkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address2.EncodeAddress(): {key2, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, sigScript)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg, err)
				break
			}

			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript,
				scriptPkScript)
			if err != nil {
				t.Errorf("fully signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}

	// Two part multisig, sign with one key then both, check key dedup
	// correctly.
	for _, hashType := range hashTypes {
		for i := range tx.TxIn {
			msg := fmt.Sprintf("%d:%d", hashType, i)

			key1, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey for %s: %v",
					msg, err)
				break
			}

			pk1 := (*btcec.PublicKey)(&key1.PublicKey).
				SerializeCompressed()
			address1, err := rmgutil.NewAddressPubKey(pk1,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address for %s: %v",
					msg, err)
				break
			}

			key2, err := btcec.NewPrivateKey(btcec.S256())
			if err != nil {
				t.Errorf("failed to make privKey 2 for %s: %v",
					msg, err)
				break
			}

			pk2 := (*btcec.PublicKey)(&key2.PublicKey).
				SerializeCompressed()
			address2, err := rmgutil.NewAddressPubKey(pk2,
				&chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make address 2 for %s: %v",
					msg, err)
				break
			}

			pkScript, err := txscript.MultiSigScript(
				[]*rmgutil.AddressPubKey{address1, address2},
				2)
			if err != nil {
				t.Errorf("failed to make pkscript "+
					"for %s: %v", msg, err)
			}

			scriptAddr, err := rmgutil.NewAddressScriptHash(
				pkScript, &chaincfg.TestNet3Params)
			if err != nil {
				t.Errorf("failed to make p2sh addr for %s: %v",
					msg, err)
				break
			}

			scriptPkScript, err := txscript.PayToAddrScript(
				scriptAddr)
			if err != nil {
				t.Errorf("failed to make script pkscript for "+
					"%s: %v", msg, err)
				break
			}

			sigScript, err := txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], scriptPkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address1.EncodeAddress(): {key1, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, nil)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg,
					err)
				break
			}

			// Only 1 out of 2 signed, this *should* fail.
			if checkScripts(msg, tx, i, inputAmounts[i], sigScript,
				scriptPkScript) == nil {
				t.Errorf("part signed script valid for %s", msg)
				break
			}

			// Sign with the other key and merge
			sigScript, err = txscript.SignTxOutput(
				&chaincfg.TestNet3Params, tx, i, inputAmounts[i], scriptPkScript,
				hashType, mkGetKey(map[string]addressToKey{
					address1.EncodeAddress(): {key1, true},
					address2.EncodeAddress(): {key2, true},
				}), mkGetScript(map[string][]byte{
					scriptAddr.EncodeAddress(): pkScript,
				}), nil, sigScript)
			if err != nil {
				t.Errorf("failed to sign output %s: %v", msg, err)
				break
			}

			// Now we should pass.
			err = checkScripts(msg, tx, i, inputAmounts[i], sigScript,
				scriptPkScript)
			if err != nil {
				t.Errorf("fully signed script invalid for "+
					"%s: %v", msg, err)
				break
			}
		}
	}
}

type tstInput struct {
	txout              *wire.TxOut
	sigscriptGenerates bool
	inputValidates     bool
	indexOutOfRange    bool
}

type tstSigScript struct {
	name               string
	inputs             []tstInput
	hashType           txscript.SigHashType
	compress           bool
	scriptAtWrongIndex bool
}

var coinbaseOutPoint = &wire.OutPoint{
	Index: (1 << 32) - 1,
}

// Pregenerated private key, with associated public key and pkScripts
// for the uncompressed and compressed hash160.
var (
	privKeyD = []byte{0x6b, 0x0f, 0xd8, 0xda, 0x54, 0x22, 0xd0, 0xb7,
		0xb4, 0xfc, 0x4e, 0x55, 0xd4, 0x88, 0x42, 0xb3, 0xa1, 0x65,
		0xac, 0x70, 0x7f, 0x3d, 0xa4, 0x39, 0x5e, 0xcb, 0x3b, 0xb0,
		0xd6, 0x0e, 0x06, 0x92}
	pubkeyX = []byte{0xb2, 0x52, 0xf0, 0x49, 0x85, 0x78, 0x03, 0x03, 0xc8,
		0x7d, 0xce, 0x51, 0x7f, 0xa8, 0x69, 0x0b, 0x91, 0x95, 0xf4,
		0xf3, 0x5c, 0x26, 0x73, 0x05, 0x05, 0xa2, 0xee, 0xbc, 0x09,
		0x38, 0x34, 0x3a}
	pubkeyY = []byte{0xb7, 0xc6, 0x7d, 0xb2, 0xe1, 0xff, 0xc8, 0x43, 0x1f,
		0x63, 0x32, 0x62, 0xaa, 0x60, 0xc6, 0x83, 0x30, 0xbd, 0x24,
		0x7e, 0xef, 0xdb, 0x6f, 0x2e, 0x8d, 0x56, 0xf0, 0x3c, 0x9f,
		0x6d, 0xb6, 0xf8}
	uncompressedPkScript = []byte{0x76, 0xa9, 0x14, 0xd1, 0x7c, 0xb5,
		0xeb, 0xa4, 0x02, 0xcb, 0x68, 0xe0, 0x69, 0x56, 0xbf, 0x32,
		0x53, 0x90, 0x0e, 0x0a, 0x86, 0xc9, 0xfa, 0x88, 0xac}
	compressedPkScript = []byte{0x76, 0xa9, 0x14, 0x27, 0x4d, 0x9f, 0x7f,
		0x61, 0x7e, 0x7c, 0x7a, 0x1c, 0x1f, 0xb2, 0x75, 0x79, 0x10,
		0x43, 0x65, 0x68, 0x27, 0x9d, 0x86, 0x88, 0xac}
	shortPkScript = []byte{0x76, 0xa9, 0x14, 0xd1, 0x7c, 0xb5,
		0xeb, 0xa4, 0x02, 0xcb, 0x68, 0xe0, 0x69, 0x56, 0xbf, 0x32,
		0x53, 0x90, 0x0e, 0x0a, 0x88, 0xac}
	uncompressedAddrStr = "1L6fd93zGmtzkK6CsZFVVoCwzZV3MUtJ4F"
	compressedAddrStr   = "14apLppt9zTq6cNw8SDfiJhk9PhkZrQtYZ"
)

// Pretend output amounts.
const coinbaseVal = 2500000000
const fee = 5000000

var sigScriptTests = []tstSigScript{
	{
		name: "one input uncompressed",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           txscript.SigHashAll,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "two inputs uncompressed",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
			{
				txout:              wire.NewTxOut(coinbaseVal+fee, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           txscript.SigHashAll,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "one input compressed",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, compressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           txscript.SigHashAll,
		compress:           true,
		scriptAtWrongIndex: false,
	},
	{
		name: "two inputs compressed",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, compressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
			{
				txout:              wire.NewTxOut(coinbaseVal+fee, compressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           txscript.SigHashAll,
		compress:           true,
		scriptAtWrongIndex: false,
	},
	{
		name: "hashType SigHashNone",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           txscript.SigHashNone,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "hashType SigHashSingle",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           txscript.SigHashSingle,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "hashType SigHashAnyoneCanPay",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           txscript.SigHashAnyOneCanPay,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "hashType non-standard",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           0x04,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "invalid compression",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     false,
				indexOutOfRange:    false,
			},
		},
		hashType:           txscript.SigHashAll,
		compress:           true,
		scriptAtWrongIndex: false,
	},
	{
		name: "short PkScript",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, shortPkScript),
				sigscriptGenerates: false,
				indexOutOfRange:    false,
			},
		},
		hashType:           txscript.SigHashAll,
		compress:           false,
		scriptAtWrongIndex: false,
	},
	{
		name: "valid script at wrong index",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
			{
				txout:              wire.NewTxOut(coinbaseVal+fee, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           txscript.SigHashAll,
		compress:           false,
		scriptAtWrongIndex: true,
	},
	{
		name: "index out of range",
		inputs: []tstInput{
			{
				txout:              wire.NewTxOut(coinbaseVal, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
			{
				txout:              wire.NewTxOut(coinbaseVal+fee, uncompressedPkScript),
				sigscriptGenerates: true,
				inputValidates:     true,
				indexOutOfRange:    false,
			},
		},
		hashType:           txscript.SigHashAll,
		compress:           false,
		scriptAtWrongIndex: true,
	},
}

// Test the sigscript generation for valid and invalid inputs, all
// hashTypes, and with and without compression.  This test creates
// sigscripts to spend fake coinbase inputs, as sigscripts cannot be
// created for the MsgTxs in txTests, since they come from the blockchain
// and we don't have the private keys.
func TestSignatureScript(t *testing.T) {
	t.Parallel()

	privKey, _ := btcec.PrivKeyFromBytes(btcec.S256(), privKeyD)

nexttest:
	for i := range sigScriptTests {
		tx := wire.NewMsgTx()

		output := wire.NewTxOut(500, []byte{txscript.OP_RETURN})
		tx.AddTxOut(output)

		for range sigScriptTests[i].inputs {
			txin := wire.NewTxIn(coinbaseOutPoint, nil)
			tx.AddTxIn(txin)
		}

		var script []byte
		var err error
		for j := range tx.TxIn {
			var idx int
			if sigScriptTests[i].inputs[j].indexOutOfRange {
				t.Errorf("at test %v", sigScriptTests[i].name)
				idx = len(sigScriptTests[i].inputs)
			} else {
				idx = j
			}
			script, err = txscript.SignatureScript(tx, idx,
				sigScriptTests[i].inputs[j].txout.PkScript,
				sigScriptTests[i].hashType, privKey,
				sigScriptTests[i].compress)

			if (err == nil) != sigScriptTests[i].inputs[j].sigscriptGenerates {
				if err == nil {
					t.Errorf("passed test '%v' incorrectly",
						sigScriptTests[i].name)
				} else {
					t.Errorf("failed test '%v': %v",
						sigScriptTests[i].name, err)
				}
				continue nexttest
			}
			if !sigScriptTests[i].inputs[j].sigscriptGenerates {
				// done with this test
				continue nexttest
			}

			tx.TxIn[j].SignatureScript = script
		}

		// If testing using a correct sigscript but for an incorrect
		// index, use last input script for first input.  Requires > 0
		// inputs for test.
		if sigScriptTests[i].scriptAtWrongIndex {
			tx.TxIn[0].SignatureScript = script
			sigScriptTests[i].inputs[0].inputValidates = false
		}

		// Validate tx input scripts
		scriptFlags := txscript.ScriptBip16 | txscript.ScriptVerifyDERSignatures
		for j := range tx.TxIn {
			vm, err := txscript.NewEngine(sigScriptTests[i].
				inputs[j].txout.PkScript, tx, j, scriptFlags, nil, nil, 0)
			if err != nil {
				t.Errorf("cannot create script vm for test %v: %v",
					sigScriptTests[i].name, err)
				continue nexttest
			}
			err = vm.Execute()
			if (err == nil) != sigScriptTests[i].inputs[j].inputValidates {
				if err == nil {
					t.Errorf("passed test '%v' validation incorrectly: %v",
						sigScriptTests[i].name, err)
				} else {
					t.Errorf("failed test '%v' validation: %v",
						sigScriptTests[i].name, err)
				}
				continue nexttest
			}
		}
	}
}
