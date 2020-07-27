package keystore

import (
	"encoding/binary"
	"errors"
	"fmt"

	"massnet.org/mass-wallet/masswallet/db"
)

var (
	keystoreVersionName = []byte("kver")

	//
	masterKMPubKeyName = []byte("mkmpub")

	// Crypto related key names (main bucket).
	masterPrivKeyName    = []byte("mpriv")
	masterPubKeyName     = []byte("mpub")
	cryptoPrivKeyName    = []byte("cpriv")
	cryptoPubKeyName     = []byte("cpub")
	cryptoEntropyKeyName = []byte("cent")

	entropyEncKeyName = []byte("ent")

	// masterHDPrivName is the name of the key that stores the master HD
	// private key. This key is encrypted with the master private crypto
	// encryption key. This resides under the main bucket.
	masterHDPrivName = []byte("mhdpriv")

	// masterHDPubName is the name of the key that stores the master HD
	// public key. This key is encrypted with the master public crypto
	// encryption key. This reside under the main bucket.
	masterHDPubName = []byte("mhdpub")

	// account usage
	accountUsageName = []byte("account")
	// coin type
	coinTypeName = []byte("coinType")
	// remark
	remarkName = []byte("remark")
	//branch
	externalBranchPubKeyName = []byte("exbPubKey")
	internalBranchPubKeyName = []byte("inbPubKey")
	externalChildNumName     = []byte("exChildNum")
	internalChildNumName     = []byte("inChildNum")
)

// accountType represents a type of address stored in the database.
type accountType uint8

// These constants define the various supported account types.
const (
	// accountDefault is the current "default" account type within the
	// database. This is an account that re-uses the key derivation schema
	// of BIP0044-like accounts.
	accountMASS accountType = 0 // not iota as they need to be stable
)

// dbAccountRow houses information stored about an account in the database.
type dbAccountRow struct {
	acctType accountType
	rawData  []byte // Varies based on account type field.
}

// dbHDAccountKey houses additional information stored about a default
// BIP0044-like account in the database.
type dbHDAccountKey struct {
	dbAccountRow
	pubKeyEncrypted  []byte
	privKeyEncrypted []byte
}

//
type pubkeyAndPath struct {
	branch    uint32
	index     uint32
	pubkeyEnc []byte
}

// uint32ToBytes converts a 32 bit unsigned integer into a 4-byte slice in
// little-endian order: 1 -> [1 0 0 0].
func uint32ToBytes(number uint32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, number)
	return buf
}

// putMasterKeyParams stores the master key parameters needed to derive them to
// the database.  Either parameter can be nil in which case no value is
// written for the parameter.
func putMasterKeyParams(b db.Bucket, pubParams, privParams []byte) error {
	if privParams != nil {
		err := b.Put(masterPrivKeyName, privParams)
		if err != nil {
			return fmt.Errorf("failed to store master private key parameters: %v", err)
		}
	}

	if pubParams != nil {
		err := b.Put(masterPubKeyName, pubParams)
		if err != nil {
			return fmt.Errorf("failed to store master public key parameters: %v", err)
		}
	}

	return nil
}

// fetchMasterKeyParams loads the master key parameters needed to derive them
// (when given the correct user-supplied passphrase) from the database.  Either
// returned value can be nil, but in practice only the private key params will
// be nil for a watching-only database.
func fetchMasterKeyParams(b db.Bucket) ([]byte, []byte, error) {
	// Load the master public key parameters.  Required.
	val, err := b.Get(masterPubKeyName)
	if err != nil {
		return nil, nil, err
	}
	if val == nil {
		str := "required master public key parameters not stored in " +
			"database"
		return nil, nil, errors.New(str)
	}
	pubParams := make([]byte, len(val))
	copy(pubParams, val)

	// Load the master private key parameters if they were stored.
	var privParams []byte
	val, err = b.Get(masterPrivKeyName)
	if err != nil {
		return nil, nil, err
	}
	if val != nil {
		privParams = make([]byte, len(val))
		copy(privParams, val)
	}

	return pubParams, privParams, nil
}

func putVersion(b db.Bucket, version uint8) error {
	buf := make([]byte, 1)
	buf[0] = version
	err := b.Put(keystoreVersionName, buf)
	if err != nil {
		return fmt.Errorf("failed to store keystore version, %v", err)
	}
	return nil
}

func fetchVersion(b db.Bucket) (uint8, error) {
	val, err := b.Get(keystoreVersionName)
	if err != nil || len(val) == 0 {
		return 0, err
	}
	return val[0], nil
}

func putEntropy(b db.Bucket, entropyEnc []byte) error {
	err := b.Put(entropyEncKeyName, entropyEnc)
	if err != nil {
		return fmt.Errorf("failed to store encrypted entropy, %v", err)
	}
	return nil
}

func fetchEntropy(b db.Bucket) ([]byte, error) {
	val, err := b.Get(entropyEncKeyName)
	if err != nil {
		return nil, err
	}
	return val, nil
}

// putCryptoKeys stores the encrypted crypto keys which are in turn used to
// protect the extended and imported keys.  Either parameter can be nil in
// which case no value is written for the parameter.
func putCryptoKeys(b db.Bucket, pubKeyEncrypted, privKeyEncrypted,
	entropyKeyEncrypted []byte) error {

	if pubKeyEncrypted != nil {
		err := b.Put(cryptoPubKeyName, pubKeyEncrypted)
		if err != nil {
			return fmt.Errorf("failed to store encrypted crypto public key: %v", err)
		}
	}

	if privKeyEncrypted != nil {
		err := b.Put(cryptoPrivKeyName, privKeyEncrypted)
		if err != nil {
			return fmt.Errorf("failed to store encrypted crypto private key: %v", err)
		}
	}

	if entropyKeyEncrypted != nil {
		err := b.Put(cryptoEntropyKeyName, entropyKeyEncrypted)
		if err != nil {
			return fmt.Errorf("failed to store encrypted crypto script key: %v", err)
		}
	}

	return nil
}

// fetchCryptoKeys loads the encrypted crypto keys which are in turn used to
// protect the extended keys, imported keys, and scripts.  Any of the returned
// values can be nil, but in practice only the crypto private and script keys
// will be nil for a watching-only database.
func fetchCryptoKeys(b db.Bucket) ([]byte, []byte, []byte, error) {
	// Load the crypto public key parameters.  Required.
	val, err := b.Get(cryptoPubKeyName)
	if err != nil {
		return nil, nil, nil, err
	}
	if val == nil {
		str := "required encrypted crypto public not stored in database"
		return nil, nil, nil, errors.New(str)
	}
	pubKey := make([]byte, len(val))
	copy(pubKey, val)

	// Load the crypto private key parameters if they were stored.
	var privKey []byte
	val, err = b.Get(cryptoPrivKeyName)
	if val != nil {
		privKey = make([]byte, len(val))
		copy(privKey, val)
	}

	// Load the crypto script key parameters if they were stored.
	var entropyKey []byte
	val, err = b.Get(cryptoEntropyKeyName)
	if val != nil {
		entropyKey = make([]byte, len(val))
		copy(entropyKey, val)
	}

	return pubKey, privKey, entropyKey, nil
}

// deserializeAccountRow deserializes the passed serialized account information.
// This is used as a common base for the various account types to deserialize
// the common parts.
func deserializeAccountRow(accountID []byte, serializedAccount []byte) (*dbAccountRow, error) {
	// The serialized account format is:
	//   <acctType><rdlen><rawdata>
	//
	// 1 byte acctType + 4 bytes raw data length + raw data

	// Given the above, the length of the entry must be at a minimum
	// the constant value sizes.
	if len(serializedAccount) < 5 {
		str := fmt.Sprintf("malformed serialized account for key %x",
			accountID)
		return nil, errors.New(str)
	}

	row := dbAccountRow{}
	row.acctType = accountType(serializedAccount[0])
	rdlen := binary.LittleEndian.Uint32(serializedAccount[1:5])
	row.rawData = make([]byte, rdlen)
	copy(row.rawData, serializedAccount[5:5+rdlen])

	return &row, nil
}

// serializeAccountRow returns the serialization of the passed account row.
func serializeAccountRow(row *dbAccountRow) []byte {
	// The serialized account format is:
	//   <acctType><rdlen><rawdata>
	//
	// 1 byte acctType + 4 bytes raw data length + raw data
	rdlen := len(row.rawData)
	buf := make([]byte, 5+rdlen)
	buf[0] = byte(row.acctType)
	binary.LittleEndian.PutUint32(buf[1:5], uint32(rdlen))
	copy(buf[5:5+rdlen], row.rawData)
	return buf
}

// deserializeHDAccountKey deserializes the raw data from the passed
// account row as a BIP0044-like account.
func deserializeHDAccountKey(accountID []byte, row *dbAccountRow) (*dbHDAccountKey, error) {
	// The serialized BIP0044 account raw data format is:
	//   <encpubkeylen><encpubkey><encprivkeylen><encprivkey><nextextidx>
	//   <nextintidx><namelen><name>
	//
	// 4 bytes encrypted pubkey len + encrypted pubkey + 4 bytes encrypted
	// privkey len + encrypted privkey + 4 bytes next external index +
	// 4 bytes next internal index + 4 bytes name len + name

	// Given the above, the length of the entry must be at a minimum
	// the constant value sizes.
	if len(row.rawData) < 8 {
		str := fmt.Sprintf("malformed serialized bip0044 account for "+
			"key %x", accountID)
		return nil, errors.New(str)
	}

	retRow := dbHDAccountKey{
		dbAccountRow: *row,
	}

	pubLen := binary.LittleEndian.Uint32(row.rawData[0:4])
	retRow.pubKeyEncrypted = make([]byte, pubLen)
	copy(retRow.pubKeyEncrypted, row.rawData[4:4+pubLen])
	offset := 4 + pubLen
	privLen := binary.LittleEndian.Uint32(row.rawData[offset : offset+4])
	offset += 4
	retRow.privKeyEncrypted = make([]byte, privLen)
	copy(retRow.privKeyEncrypted, row.rawData[offset:offset+privLen])

	return &retRow, nil
}

// serializeHDAccountKey returns the serialization of the raw data field
// for a BIP0044-like account.
func serializeHDAccountKey(encryptedPubKey, encryptedPrivKey []byte) []byte {

	// The serialized BIP0044 account raw data format is:
	//   <encpubkeylen><encpubkey><encprivkeylen><encprivkey><nextextidx>
	//   <nextintidx><namelen><name>
	//
	// 4 bytes encrypted pubkey len + encrypted pubkey + 4 bytes encrypted
	// privkey len + encrypted privkey + 4 bytes next external index +
	// 4 bytes next internal index + 4 bytes name len + name
	pubLen := uint32(len(encryptedPubKey))
	privLen := uint32(len(encryptedPrivKey))
	rawData := make([]byte, 8+pubLen+privLen)
	binary.LittleEndian.PutUint32(rawData[0:4], pubLen)
	copy(rawData[4:4+pubLen], encryptedPubKey)
	offset := 4 + pubLen
	binary.LittleEndian.PutUint32(rawData[offset:offset+4], privLen)
	offset += 4
	copy(rawData[offset:offset+privLen], encryptedPrivKey)
	return rawData
}

func fetchAccountUsage(b db.Bucket) (uint32, error) {
	val, _ := b.Get(accountUsageName)
	if val == nil {
		str := "required account usage not stored in " +
			"database"
		return 0, errors.New(str)
	}
	return binary.LittleEndian.Uint32(val), nil
}

func putAccountUsage(b db.Bucket, account uint32) error {
	err := b.Put(accountUsageName, uint32ToBytes(account))
	if err != nil {
		str := fmt.Sprintf("failed to store account usage %d", account)
		return errors.New(str)
	}
	return nil
}

// putAccountRow stores the provided account information to the database.  This
// is used a common base for storing the various account types.
func putAccountRow(b db.Bucket, scope *KeyScope,
	accountUsage uint32, accountInfo *dbAccountRow) error {
	data := serializeAccountRow(accountInfo)

	// Write the serialized value keyed by the account number.
	err := b.Put(uint32ToBytes(accountUsage), data)
	if err != nil {
		str := fmt.Sprintf("failed to store account %d", accountUsage)
		return errors.New(str)
	}
	return nil
}

func putCoinType(b db.Bucket, coin uint32) error {
	return b.Put(coinTypeName, uint32ToBytes(coin))
}

func fetchCoinType(b db.Bucket) (uint32, error) {
	val, err := b.Get(coinTypeName)
	if err != nil {
		return 0, err
	}
	if val == nil {
		str := "required coin type not stored in database"
		return 0, errors.New(str)
	}
	return binary.LittleEndian.Uint32(val), nil
}

// putAccountInfo stores the provided account information to the database.
func putAccountInfo(b db.Bucket, scope *KeyScope,
	account uint32, encryptedPubKey, encryptedPrivKey []byte) error {

	rawData := serializeHDAccountKey(encryptedPubKey, encryptedPrivKey)

	err := putAccountUsage(b, account)
	if err != nil {
		return err
	}

	acctRow := dbAccountRow{
		acctType: accountMASS,
		rawData:  rawData,
	}
	if err := putAccountRow(b, scope, account, &acctRow); err != nil {
		return err
	}

	return nil
}

// fetchAccountInfo loads information about the passed account from the
// database.
func fetchAccountInfo(b db.Bucket, account uint32) (interface{}, error) {
	accountID := uint32ToBytes(account)
	data, _ := b.Get(accountID)
	if data == nil {
		str := fmt.Sprintf("account %d not found", account)
		return nil, errors.New(str)
	}

	row, err := deserializeAccountRow(accountID, data)
	if err != nil {
		return nil, err
	}

	switch row.acctType {
	case accountMASS:
		return deserializeHDAccountKey(accountID, row)
	}

	str := fmt.Sprintf("unsupported account type '%d'", row.acctType)
	return nil, errors.New(str)
}

// put account PubKey into seed bucket
func putAccountID(b db.Bucket, acctAddr0 []byte) error {
	return b.Put(acctAddr0, []byte{0})
}

func fetchAccountID(b db.Bucket) (seeds [][]byte, err error) {
	entries, err := b.GetByPrefix([]byte{})
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		seeds = append(seeds, entry.Key)
	}
	return
}

func deleteAccountID(b db.Bucket, acctID []byte) error {
	return b.Delete(acctID)
}

func putRemark(b db.Bucket, remark []byte) error {
	return b.Put(remarkName, remark)
}

func deleteRemark(b db.Bucket) error {
	return b.Delete(remarkName)
}

func fetchRemark(b db.Bucket) ([]byte, error) {
	remark, err := b.Get(remarkName)
	if err != nil {
		return nil, err
	}
	return remark, nil
}

// branch
func putBranchPubKeys(b db.Bucket, encryptedInternalKey []byte, encryptedExternalKey []byte) error {
	err := b.Put(externalBranchPubKeyName, encryptedExternalKey)
	if err != nil {
		return fmt.Errorf("failed to store branchKeys: %v", err)
	}
	err = b.Put(internalBranchPubKeyName, encryptedInternalKey)
	if err != nil {
		return fmt.Errorf("failed to store branchKeys: %v", err)
	}

	return nil
}

func fetchBranchPubKeys(b db.Bucket) ([]byte, []byte, error) {
	exBrPub, err := b.Get(externalBranchPubKeyName)
	if err != nil {
		return nil, nil, err
	}
	if exBrPub == nil {
		str := "required encrypted external branch public key not stored in database"
		return nil, nil, errors.New(str)
	}

	inBrPub, err := b.Get(internalBranchPubKeyName)
	if err != nil {
		return nil, nil, err
	}
	if inBrPub == nil {
		str := "required encrypted internal branch public key not stored in database"
		return nil, nil, errors.New(str)
	}
	return inBrPub, exBrPub, nil
}

func initBranchChildNum(b db.Bucket) error {
	err := b.Put(externalChildNumName, uint32ToBytes(0))
	if err != nil {
		return fmt.Errorf("failed to init externalChildNumName: %v", err)
	}
	err = b.Put(internalChildNumName, uint32ToBytes(0))
	if err != nil {
		return fmt.Errorf("failed to init internalChildNumName: %v", err)
	}

	return nil
}

// set the next Index
func updateChildNum(b db.Bucket, internal bool, nextIndex uint32) error {
	var key []byte
	if internal {
		key = internalChildNumName
	} else {
		key = externalChildNumName
	}
	err := b.Put(key, uint32ToBytes(nextIndex))
	if err != nil {
		return fmt.Errorf("failed to update branchChildNum: %v", err)
	}
	return nil
}

func fetchChildNum(b db.Bucket) (uint32, uint32, error) {
	exChildNum, err := b.Get(externalChildNumName)
	if err != nil {
		return 0, 0, err
	}
	if exChildNum == nil {
		str := "required encrypted external branch child number not stored in database"
		return 0, 0, errors.New(str)
	}

	inChildNum, err := b.Get(internalChildNumName)
	if err != nil {
		return 0, 0, err
	}
	if inChildNum == nil {
		str := "required encrypted internal branch child number not stored in database"
		return 0, 0, errors.New(str)
	}
	return binary.LittleEndian.Uint32(inChildNum), binary.LittleEndian.Uint32(exChildNum), nil
}

func getChildNum(b db.Bucket, internal bool) (uint32, error) {
	var key []byte
	if internal {
		key = internalChildNumName
	} else {
		key = externalChildNumName
	}
	childNum, err := b.Get(key)
	if err != nil {
		return 0, fmt.Errorf("failed to get %d: %v", key, err)
	}

	return binary.LittleEndian.Uint32(childNum), nil
}

// put encrypted pubKey into db when new address
func putEncryptedPubKey(b db.Bucket, branch, index uint32, pubKey []byte) error {
	key := make([]byte, 8, 8)
	copy(key[:4], uint32ToBytes(branch))
	copy(key[4:8], uint32ToBytes(index))
	return b.Put(key, pubKey)
}

func fetchEncryptedPubKey(b db.Bucket) ([]*pubkeyAndPath, error) {
	entries, err := b.GetByPrefix([]byte{})
	if err != nil {
		return nil, err
	}
	pks := make([]*pubkeyAndPath, 0, len(entries))
	for _, entry := range entries {
		key := entry.Key
		pkp := &pubkeyAndPath{
			branch:    binary.LittleEndian.Uint32(key[:4]),
			index:     binary.LittleEndian.Uint32(key[4:8]),
			pubkeyEnc: entry.Value,
		}
		pks = append(pks, pkp)
	}
	return pks, nil
}
