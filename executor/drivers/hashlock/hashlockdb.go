package hashlock

//database opeartion for execs hashlock
import (
	"bytes"
	"encoding/json"
	"fmt"

	log "github.com/inconshreveable/log15"
	"gitlab.33.cn/chain33/chain33/account"
	"gitlab.33.cn/chain33/chain33/common"
	dbm "gitlab.33.cn/chain33/chain33/common/db"
	"gitlab.33.cn/chain33/chain33/types"
)

var hlog = log.New("module", "hashlock.db")

const (
	Hashlock_Locked   = 1
	Hashlock_Unlocked = 2
	Hashlock_Sent     = 3
)

type HashlockDB struct {
	types.Hashlock
}

func NewHashlockDB(id []byte, returnWallet string, toAddress string, blocktime int64, amount int64, time int64) *HashlockDB {
	h := &HashlockDB{}
	h.HashlockId = id
	h.ReturnAddress = returnWallet
	h.ToAddress = toAddress
	h.CreateTime = blocktime
	h.Status = Hashlock_Locked
	h.Amount = amount
	h.Frozentime = time
	return h
}

func (h *HashlockDB) GetKVSet() (kvset []*types.KeyValue) {
	value := types.Encode(&h.Hashlock)

	kvset = append(kvset, &types.KeyValue{HashlockKey(h.HashlockId), value})
	return kvset
}

func (h *HashlockDB) Save(db dbm.KVDB) {
	set := h.GetKVSet()
	for i := 0; i < len(set); i++ {
		db.Set(set[i].GetKey(), set[i].Value)
	}
}

func HashlockKey(id []byte) (key []byte) {
	key = append(key, []byte("mavl-hashlock-")...)
	key = append(key, id...)
	return key
}

type HashlockAction struct {
	coinsAccount *account.AccountDB
	db           dbm.KVDB
	txhash       []byte
	fromaddr     string
	blocktime    int64
	height       int64
	execaddr     string
}

func NewHashlockAction(h *Hashlock, tx *types.Transaction, execaddr string) *HashlockAction {
	hash := tx.Hash()
	fromaddr := account.PubKeyToAddress(tx.GetSignature().GetPubkey()).String()
	return &HashlockAction{h.GetCoinsAccount(), h.GetDB(), hash, fromaddr, h.GetBlockTime(), h.GetHeight(), execaddr}
}

func (action *HashlockAction) Hashlocklock(hlock *types.HashlockLock) (*types.Receipt, error) {

	var logs []*types.ReceiptLog
	var kv []*types.KeyValue

	//不存在相同的hashlock，假定采用sha256
	_, err := readHashlock(action.db, hlock.Hash)
	if err != types.ErrNotFound {
		hlog.Error("Hashlocklock", "hlock.Hash repeated", hlock.Hash)
		return nil, types.ErrHashlockReapeathash
	}

	h := NewHashlockDB(hlock.Hash, action.fromaddr, hlock.ToAddress, action.blocktime, hlock.Amount, hlock.Time)
	//冻结子账户资金
	receipt, err := action.coinsAccount.ExecFrozen(action.fromaddr, action.execaddr, hlock.Amount)

	if err != nil {
		hlog.Error("Hashlocklock.Frozen", "addr", action.fromaddr, "execaddr", action.execaddr, "amount", hlock.Amount)
		return nil, err
	}

	h.Save(action.db)
	logs = append(logs, receipt.Logs...)
	kv = append(kv, receipt.KV...)
	//logs = append(logs, h.GetReceiptLog())
	kv = append(kv, h.GetKVSet()...)

	receipt = &types.Receipt{types.ExecOk, kv, logs}
	return receipt, nil
}

func (action *HashlockAction) Hashlockunlock(unlock *types.HashlockUnlock) (*types.Receipt, error) {

	var logs []*types.ReceiptLog
	var kv []*types.KeyValue

	hash, err := readHashlock(action.db, common.Sha256(unlock.Secret))
	if err != nil {
		hlog.Error("Hashlockunlock", "unlock.Secret", unlock.Secret)
		return nil, err
	}

	if hash.ReturnAddress != action.fromaddr {
		hlog.Error("Hashlockunlock.Frozen", "action.fromaddr", action.fromaddr)
		return nil, types.ErrHashlockReturnAddrss
	}

	if hash.Status != Hashlock_Locked {
		hlog.Error("Hashlockunlock", "hash.Status", hash.Status)
		return nil, types.ErrHashlockStatus
	}

	if action.blocktime-hash.GetCreateTime() < hash.Frozentime {
		hlog.Error("Hashlockunlock", "action.blocktime-hash.GetCreateTime", action.blocktime-hash.GetCreateTime())
		return nil, types.ErrTime
	}

	//different with typedef in C
	h := &HashlockDB{*hash}
	receipt, errR := action.coinsAccount.ExecActive(h.ReturnAddress, action.execaddr, h.Amount)
	if errR != nil {
		hlog.Error("ExecActive error", "ReturnAddress", h.ReturnAddress, "execaddr", action.execaddr, "amount", h.Amount)
		return nil, errR
	}

	h.Status = Hashlock_Unlocked
	h.Save(action.db)
	logs = append(logs, receipt.Logs...)
	kv = append(kv, receipt.KV...)
	//logs = append(logs, t.GetReceiptLog())
	kv = append(kv, h.GetKVSet()...)

	receipt = &types.Receipt{types.ExecOk, kv, logs}
	return receipt, nil
}

func (action *HashlockAction) Hashlocksend(send *types.HashlockSend) (*types.Receipt, error) {

	var logs []*types.ReceiptLog
	var kv []*types.KeyValue

	hash, err := readHashlock(action.db, common.Sha256(send.Secret))
	if err != nil {
		hlog.Error("Hashlocksend", "send.Secret", send.Secret)
		return nil, err
	}

	if hash.Status != Hashlock_Locked {
		hlog.Error("Hashlocksend", "hash.Status", hash.Status)
		return nil, types.ErrHashlockStatus
	}

	if action.fromaddr != hash.ToAddress {
		hlog.Error("Hashlocksend", "action.fromaddr", action.fromaddr, "hash.ToAddress", hash.ToAddress)
		return nil, types.ErrHashlockSendAddress
	}

	if action.blocktime-hash.GetCreateTime() > hash.Frozentime {
		hlog.Error("Hashlocksend", "action.blocktime-hash.GetCreateTime", action.blocktime-hash.GetCreateTime())
		return nil, types.ErrTime
	}

	//different with typedef in C
	h := &HashlockDB{*hash}
	receipt, errR := action.coinsAccount.ExecTransferFrozen(h.ReturnAddress, h.ToAddress, action.execaddr, h.Amount)
	if errR != nil {
		hlog.Error("ExecTransferFrozen error", "ReturnAddress", h.ReturnAddress, "ToAddress", h.ToAddress, "execaddr", action.execaddr, "amount", h.Amount)
		return nil, errR
	}
	h.Status = Hashlock_Sent
	h.Save(action.db)
	logs = append(logs, receipt.Logs...)
	kv = append(kv, receipt.KV...)
	//logs = append(logs, t.GetReceiptLog())
	kv = append(kv, h.GetKVSet()...)

	receipt = &types.Receipt{types.ExecOk, kv, logs}
	return receipt, nil
}

func readHashlock(db dbm.KVDB, id []byte) (*types.Hashlock, error) {
	data, err := db.Get(HashlockKey(id))
	if err != nil {
		return nil, err
	}
	var hashlock types.Hashlock
	//decode
	err = types.Decode(data, &hashlock)
	if err != nil {
		return nil, err
	}
	return &hashlock, nil
}

func checksecret(secret []byte, hashresult []byte) bool {
	return bytes.Equal(common.Sha256(secret), hashresult)
}

func NewHashlockquery() *types.Hashlockquery {
	q := types.Hashlockquery{}
	return &q
}

//将Information转换成byte类型，使输出为kv模式
func GeHashReciverKV(hashlockId []byte, information *types.Hashlockquery) *types.KeyValue {
	clog.Error("GeHashReciverKV action")
	infor := types.Hashlockquery{information.Time, information.Status, information.Amount, information.CreateTime, information.CurrentTime}
	clog.Error("GeHashReciverKV action", "Status", information.Status)
	reciver, err := json.Marshal(infor)
	if err == nil {
		fmt.Println("成功转换为json格式")
	} else {
		fmt.Println(err)
	}
	clog.Error("GeHashReciverKV action", "reciver", reciver)
	kv := &types.KeyValue{hashlockId, reciver}
	clog.Error("GeHashReciverKV action", "kv", kv)
	return kv
}

//从db里面根据key获取value,期间需要进行解码
func GetHashReciver(db dbm.KVDB, hashlockId []byte) (*types.Hashlockquery, error) {
	//reciver := types.Int64{}
	clog.Error("GetHashReciver action", "hashlockID", hashlockId)
	reciver := NewHashlockquery()
	hashReciver, err := db.Get(hashlockId)
	if err != nil {
		clog.Error("Get err")
		return reciver, err
	}
	fmt.Println(hashReciver)
	if hashReciver == nil {
		clog.Error("nilnilnilllllllllll")

	}
	clog.Error("hashReciver", "len", len(hashReciver))
	clog.Error("GetHashReciver", "hashReciver", hashReciver)
	err = json.Unmarshal(hashReciver, reciver)
	if err != nil {
		clog.Error("hashReciver Unmarshal")
		return nil, err
	}
	clog.Error("GetHashReciver", "reciver", reciver)
	return reciver, nil
}

//将hashlockId和information都以key和value形式存入db
func SetHashReciver(db dbm.KVDB, hashlockId []byte, information *types.Hashlockquery) error {
	clog.Error("SetHashReciver action")
	kv := GeHashReciverKV(hashlockId, information)
	return db.Set(kv.Key, kv.Value)
}

//根据状态值对db中存入的数据进行更改
func UpdateHashReciver(cachedb dbm.KVDB, hashlockId []byte, information types.Hashlockquery) (*types.KeyValue, error) {
	clog.Error("UpdateHashReciver", "hashlockId", hashlockId)
	recv, err := GetHashReciver(cachedb, hashlockId)
	if err != nil && err != types.ErrNotFound {
		clog.Error("UpdateHashReciver", "err", err)
		return nil, err
	}
	fmt.Println(recv)
	clog.Error("UpdateHashReciver", "recv", recv)
	//	clog.Error("UpdateHashReciver", "Status", information.Status)
	//	var action types.HashlockAction
	//当处于lock状态时，在db中是找不到的，此时需要创建并存储于db中，其他状态则能从db中找到
	if information.Status == Hashlock_Locked {
		clog.Error("UpdateHashReciver", "Hashlock_Locked", Hashlock_Locked)
		if err == types.ErrNotFound {
			clog.Error("UpdateHashReciver", "Hashlock_Locked")
			recv.Time = information.Time
			recv.Status = Hashlock_Locked //1
			recv.Amount = information.Amount
			recv.CreateTime = information.CreateTime
			//			clog.Error("UpdateHashReciver", "Statuslock", recv.Status)
			clog.Error("UpdateHashReciver", "recv", recv)
		}
	} else if information.Status == Hashlock_Unlocked {
		clog.Error("UpdateHashReciver", "Hashlock_Unlocked", Hashlock_Unlocked)
		if err == nil {
			recv.Status = Hashlock_Unlocked //2
			//			clog.Error("UpdateHashReciver", "Statusunlock", recv.Status)
			clog.Error("UpdateHashReciver", "recv", recv)
		}
	} else if information.Status == Hashlock_Sent {
		clog.Error("UpdateHashReciver", "Hashlock_Sent", Hashlock_Sent)
		if err == nil {
			recv.Status = Hashlock_Sent //3
			//			clog.Error("UpdateHashReciver", "Statussend", recv.Status)
			clog.Error("UpdateHashReciver", "recv", recv)
		}
	}
	SetHashReciver(cachedb, hashlockId, recv)
	//keyvalue
	return GeHashReciverKV(hashlockId, recv), nil
}

//根据hashlockid获取相关信息
func (n *Hashlock) GetTxsByHashlockId(hashlockId []byte, differTime int64) (types.Message, error) {
	clog.Error("GetTxsByHashlockId action")
	db := n.GetLocalDB()
	query, err := GetHashReciver(db, hashlockId)
	if err != nil {
		return nil, err
	}
	query.CurrentTime = differTime
	//	qresult := types.Hashlockquery{query.Time, query.Status, query.Amount, query.CreateTime, currentTime}
	return query, nil
}