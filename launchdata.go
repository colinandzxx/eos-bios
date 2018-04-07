package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/eoscanada/eos-go"
	"github.com/eoscanada/eos-go/ecc"
)

type LaunchData struct {
	LaunchBitcoinBlockHeight    int    `json:"launch_btc_block_height"`
	OpeningBalancesSnapshotHash string `json:"opening_balances_snapshot_hash"`
	ContractHashes              struct {
		BIOS   string `json:"bios"`
		System string `json:"system"`
		Msig   string `json:"msig"`
		Token  string `json:"token"`
	} `json:"contract_hashes"`

	Producers []*ProducerDef `json:"producers"`
}
type ProducerDef struct {
	// AccountName is the account we want to have created on the blockchain by the BIOS Boot node.
	AccountName eos.AccountName `json:"account_name"`

	// Authority is the original authority the Boot node will register
	// on that account. This allows teams to do their key ceremony a
	// few days before, and avoids a bootstrapping issue if we only
	// had a single public key for that account.
	Authority struct {
		Owner  eos.Authority `json:"owner"`
		Active eos.Authority `json:"active"`
	} `json:"authority"`

	// The key initially injected and used by the Appointed Block
	// Producers (if elected as such) to sign some of the first
	// blocks.
	//
	// When the ABP jumps in, it will `regproducer` with the same or a
	// different key (see Config's BlockSigningPublicKey).
	InitialBlockSigningPublicKey ecc.PublicKey `json:"initial_block_signing_public_key"`

	// KeybaseUser and PGPPublicKey are used to encrypt the Kickstart
	// Data payload, for the ABPs and followers.
	KeybaseUser  string `json:"keybase_user"`
	PGPPublicKey string `json:"pgp_public_key"`

	// OrganizationName is the block producer's name in plain text.
	OrganizationName string `json:"organization_name"`

	// Candidate producers are better off specifying a few URLs and social media properties, to avoid a single point of failure if they need to communicate with the world.
	URLs []string `json:"urls"`
}

func (p *ProducerDef) String() string {
	return fmt.Sprintf("Account: % 15s  Keybase: https://keybase.io/%s     Org: % 30s URL: %s", p.AccountName, p.KeybaseUser, p.OrganizationName, strings.Join(p.URLs, ", "))
}

// snapshotPath, codePath, abiPath string
func loadLaunchFile(filename string, config *Config) (out *LaunchData, err error) {
	cnt, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	if err := yamlUnmarshal(cnt, &out); err != nil {
		return nil, err
	}

	if out.LaunchBitcoinBlockHeight == 0 {
		return nil, fmt.Errorf("launch_btc_block_height unspecified (or 0)")
	}

	// Hash the `--opening-balance-snapshot` file, compare to `launch.
	snapshotHash, err := hashFile(config.OpeningBalances.SnapshotPath)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Hash of %q: %s\n", config.OpeningBalances.SnapshotPath, snapshotHash)

	if snapshotHash != out.OpeningBalancesSnapshotHash {
		return nil, fmt.Errorf("snapshot hash doesn't match launch data")
	}

	for name, cmp := range map[string]contractCompare{
		"bios":   newCC(config.Contracts.BIOS, out.ContractHashes.BIOS),
		"system": newCC(config.Contracts.System, out.ContractHashes.System),
		"msig":   newCC(config.Contracts.Msig, out.ContractHashes.Msig),
		"token":  newCC(config.Contracts.Token, out.ContractHashes.Token),
	} {
		// TODO: check all contracts and align on its content
		codeHash, err := hashCodeFiles(cmp.location.CodePath, cmp.location.ABIPath)
		if err != nil {
			return nil, fmt.Errorf("error hashing %q contract's code + abi: %s", name, err)
		}

		fmt.Printf("Hash of %q and %q: %s\n", cmp.location.CodePath, cmp.location.ABIPath, codeHash)

		if codeHash != cmp.hash {
			return nil, fmt.Errorf("%q contract's code hash don't match", name)
		}
	}

	// Check duplicate entries in `launch.yaml`, fail immediately.
	//    Check the `account_name`
	// Hash the eosio-system-code and eosio-system-abi files, concatenated.
	//    If check fails, print the hash.. always print the hash.

	return out, nil
}

func newCC(loc ContractLocation, hash string) contractCompare {
	return contractCompare{loc, hash}
}

type contractCompare struct {
	location ContractLocation
	hash     string
}

func hashFile(filename string) (string, error) {
	h := sha256.New()

	cnt, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}

	h.Write(cnt)

	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashCodeFiles(code, abi string) (string, error) {
	h := sha256.New()

	cnt, err := ioutil.ReadFile(code)
	if err != nil {
		return "", err
	}

	h.Write(cnt)

	h.Write([]byte(":"))

	cnt, err = ioutil.ReadFile(abi)
	if err != nil {
		return "", err
	}

	h.Write(cnt)

	return hex.EncodeToString(h.Sum(nil)), nil
}
