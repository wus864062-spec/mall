package data

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

	"mall/internal/biz"
	"mall/internal/conf"

	"github.com/go-sql-driver/mysql"
)

const mysqlTimeout = 15 * time.Second

func mysqlDSN(c *conf.Data) string {
	if v := strings.TrimSpace(os.Getenv("MALL_MYSQL_DSN")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("MYSQL_DSN")); v != "" {
		return v
	}
	if c != nil && c.Database != nil {
		return strings.TrimSpace(c.Database.Source)
	}
	return ""
}

func openMySQL(dsn string) (*sql.DB, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("mysql dsn: %w", err)
	}
	cfg.ParseTime = true
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	if cfg.Params["charset"] == "" {
		cfg.Params["charset"] = "utf8mb4"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = mysqlTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = mysqlTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = mysqlTimeout
	}
	dbName := cfg.DBName
	if dbName == "" {
		dbName = "mall"
	}
	if err := validateDBName(dbName); err != nil {
		return nil, err
	}
	cfg.DBName = ""
	admin, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), mysqlTimeout)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		_ = admin.Close()
		return nil, fmt.Errorf("mysql ping: %w", err)
	}
	q := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", dbName)
	if _, err := admin.ExecContext(ctx, q); err != nil {
		_ = admin.Close()
		return nil, fmt.Errorf("create database: %w", err)
	}
	_ = admin.Close()

	cfg.DBName = dbName
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)
	pingCtx, pingCancel := context.WithTimeout(context.Background(), mysqlTimeout)
	defer pingCancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("mysql ping %s: %w", dbName, err)
	}
	if err := migrateMySQL(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func validateDBName(name string) error {
	if name == "" {
		return fmt.Errorf("empty mysql database name")
	}
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			continue
		}
		return fmt.Errorf("invalid mysql database name")
	}
	return nil
}

func migrateMySQL(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS mall_meta (
			k VARCHAR(64) NOT NULL PRIMARY KEY,
			v VARCHAR(255) NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT NOT NULL PRIMARY KEY,
			wallet_address VARCHAR(128) NOT NULL,
			nickname VARCHAR(128) NOT NULL DEFAULT '',
			avatar VARCHAR(512) NOT NULL DEFAULT '',
			invite_code VARCHAR(128) NULL,
			inviter_id BIGINT NOT NULL DEFAULT 0,
			parent_id BIGINT NOT NULL DEFAULT 0,
			side VARCHAR(16) NOT NULL DEFAULT '',
			left_id BIGINT NOT NULL DEFAULT 0,
			right_id BIGINT NOT NULL DEFAULT 0,
			chain_next_id BIGINT NOT NULL DEFAULT 0,
			occupied_pos BIGINT NOT NULL DEFAULT 0,
			perf_fen BIGINT NOT NULL DEFAULT 0,
			settled_pair_fen BIGINT NOT NULL DEFAULT 0,
			pair_cleared_right_fen BIGINT NOT NULL DEFAULT 0,
			token_version BIGINT NOT NULL DEFAULT 0,
			static_days BIGINT NOT NULL DEFAULT 0,
			static_released_days BIGINT NOT NULL DEFAULT 0,
			static_package_fen BIGINT NOT NULL DEFAULT 0,
			dynamic_reward_day VARCHAR(10) NOT NULL DEFAULT '',
			dynamic_reward_fen BIGINT NOT NULL DEFAULT 0,
			created_at BIGINT NOT NULL DEFAULT 0,
			balance_fen BIGINT NOT NULL DEFAULT 0,
			ispay_locked_micro BIGINT NOT NULL DEFAULT 0,
			ispay_free_micro BIGINT NOT NULL DEFAULT 0,
			frozen_usdt_fen BIGINT NOT NULL DEFAULT 0,
			ispay_frozen_micro BIGINT NOT NULL DEFAULT 0,
			unfreeze_remain_fen BIGINT NOT NULL DEFAULT 0,
			unfreeze_grant_day VARCHAR(10) NOT NULL DEFAULT '',
			unfreeze_granted_fen BIGINT NOT NULL DEFAULT 0,
			frozen_lots_json JSON NULL,
			locked TINYINT NOT NULL DEFAULT 0,
			skip_upline_reward TINYINT NOT NULL DEFAULT 0,
			invite_root TINYINT NOT NULL DEFAULT 0,
			UNIQUE KEY uk_wallet (wallet_address),
			UNIQUE KEY uk_invite (invite_code)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS products (
			id BIGINT NOT NULL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			price_fen BIGINT NOT NULL DEFAULT 0,
			stock BIGINT NOT NULL DEFAULT 0,
			image VARCHAR(512) NOT NULL DEFAULT '',
			category_id BIGINT NOT NULL DEFAULT 0,
			status INT NOT NULL DEFAULT 0
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS packages (
			id BIGINT NOT NULL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			amount_fen BIGINT NOT NULL DEFAULT 0,
			release_days BIGINT NOT NULL DEFAULT 0,
			image VARCHAR(512) NOT NULL DEFAULT '',
			status INT NOT NULL DEFAULT 0,
			product_id BIGINT NOT NULL DEFAULT 0,
			sort_order BIGINT NOT NULL DEFAULT 0,
			UNIQUE KEY uk_days_amount (release_days, amount_fen)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS orders (
			id BIGINT NOT NULL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			status VARCHAR(32) NOT NULL,
			total_fen BIGINT NOT NULL DEFAULT 0,
			created_at BIGINT NOT NULL DEFAULT 0,
			items_json JSON NOT NULL,
			category_id BIGINT NOT NULL DEFAULT 0,
			coins_micro BIGINT NOT NULL DEFAULT 0,
			release_days BIGINT NOT NULL DEFAULT 0,
			released_days BIGINT NOT NULL DEFAULT 0,
			KEY idx_user (user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS ledger (
			id BIGINT NOT NULL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			amount_fen BIGINT NOT NULL,
			balance_fen BIGINT NOT NULL,
			type VARCHAR(64) NOT NULL,
			ref_type VARCHAR(64) NOT NULL DEFAULT '',
			ref_id BIGINT NOT NULL DEFAULT 0,
			created_at BIGINT NOT NULL DEFAULT 0,
			tx_hash VARCHAR(66) NULL,
			KEY idx_user (user_id),
			UNIQUE KEY uk_ledger_tx_hash (tx_hash)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	ctx, cancel := context.WithTimeout(context.Background(), mysqlTimeout)
	defer cancel()
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("migrate mysql: %w", err)
		}
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS community_posts`); err != nil {
		return fmt.Errorf("migrate mysql drop community_posts: %w", err)
	}
	alters := []string{
		`ALTER TABLE users ADD COLUMN ispay_locked_micro BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN ispay_free_micro BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE orders ADD COLUMN category_id BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE orders ADD COLUMN coins_micro BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE orders ADD COLUMN release_days BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE orders ADD COLUMN released_days BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN static_days BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN static_released_days BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN static_package_fen BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN locked TINYINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN skip_upline_reward TINYINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN invite_root TINYINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN dynamic_reward_day VARCHAR(10) NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN dynamic_reward_fen BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN frozen_usdt_fen BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN ispay_frozen_micro BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN unfreeze_remain_fen BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN unfreeze_grant_day VARCHAR(10) NOT NULL DEFAULT ''`,
		`ALTER TABLE users ADD COLUMN unfreeze_granted_fen BIGINT NOT NULL DEFAULT 0`,
		`ALTER TABLE users ADD COLUMN frozen_lots_json JSON NULL`,
		`ALTER TABLE users MODIFY COLUMN invite_code VARCHAR(128) NULL`,
		`ALTER TABLE ledger ADD COLUMN tx_hash VARCHAR(66) NULL`,
	}
	for _, s := range alters {
		if _, err := db.ExecContext(ctx, s); err != nil {
			if !isDupColumn(err) {
				return fmt.Errorf("migrate mysql alter: %w", err)
			}
		}
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE ledger ADD UNIQUE KEY uk_ledger_tx_hash (tx_hash)`); err != nil {
		if !isDupIndex(err) {
			return fmt.Errorf("migrate mysql unique tx_hash: %w", err)
		}
	}
	return nil
}

func isDupColumn(err error) bool {
	if err == nil {
		return false
	}
	var me *mysql.MySQLError
	if errors.As(err, &me) && me.Number == 1060 {
		return true
	}
	return strings.Contains(err.Error(), "Duplicate column")
}

func isDupIndex(err error) bool {
	if err == nil {
		return false
	}
	var me *mysql.MySQLError
	if errors.As(err, &me) && (me.Number == 1061 || me.Number == 1068) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "Duplicate key") || strings.Contains(msg, "Multiple primary key")
}

func (d *Data) loadMySQL() error {
	ctx, cancel := context.WithTimeout(context.Background(), mysqlTimeout)
	defer cancel()

	meta := map[string]string{}
	rows, err := d.db.QueryContext(ctx, `SELECT k, v FROM mall_meta`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			_ = rows.Close()
			return err
		}
		meta[k] = v
	}
	_ = rows.Close()
	d.userSeq = atoi64(meta["user_seq"])
	d.ledgerSeq = atoi64(meta["ledger_seq"])
	d.orderSeq = atoi64(meta["order_seq"])
	d.lastPairSettleDate = meta["last_pair_settle_date"]
	d.ispayPriceFen = atoi64(meta["ispay_price_fen"])
	d.moneyScale = atoi64(meta["money_scale"])
	if _, ok := meta["withdraw_fee_percent"]; ok {
		d.withdrawFeePercent = atoi64(meta["withdraw_fee_percent"])
	} else {
		d.withdrawFeePercent = biz.WithdrawFeePercent
	}
	if _, ok := meta["min_withdraw_fen"]; ok {
		d.minWithdrawFen = atoi64(meta["min_withdraw_fen"])
	}
	if _, ok := meta["max_recharge_fen"]; ok {
		d.maxRechargeFen = atoi64(meta["max_recharge_fen"])
	}
	d.pairCaps = biz.DecodePairCapsJSON(meta["pair_caps"])
	d.ensureAssetMaps()

	urows, err := d.db.QueryContext(ctx, `SELECT id, wallet_address, nickname, avatar, invite_code, inviter_id, parent_id, side,
		left_id, right_id, chain_next_id, occupied_pos, perf_fen, settled_pair_fen, pair_cleared_right_fen, token_version,
		static_days, static_released_days, static_package_fen, dynamic_reward_day, dynamic_reward_fen, created_at, balance_fen,
		ispay_locked_micro, ispay_free_micro, frozen_usdt_fen, ispay_frozen_micro, unfreeze_remain_fen, unfreeze_grant_day, unfreeze_granted_fen, frozen_lots_json, locked, skip_upline_reward, invite_root
		FROM users`)
	if err != nil {
		return err
	}
	defer urows.Close()
	for urows.Next() {
		u := &biz.User{}
		var bal, lockedMicro, freeMicro, frozenUsdt, frozenMicro, remainFen, grantedFen, locked, skipUpline, inviteRoot int64
		var invite sql.NullString
		var lotsRaw []byte
		if err := urows.Scan(&u.ID, &u.WalletAddress, &u.Nickname, &u.Avatar, &invite, &u.InviterID, &u.ParentID, &u.Side,
			&u.LeftID, &u.RightID, &u.ChainNextID, &u.OccupiedPos, &u.PerfFen, &u.SettledPairFen, &u.PairClearedRightFen, &u.TokenVersion,
			&u.StaticDays, &u.StaticReleasedDays, &u.StaticPackageFen, &u.DynamicRewardDay, &u.DynamicRewardFen, &u.CreatedAt, &bal, &lockedMicro, &freeMicro, &frozenUsdt, &frozenMicro, &remainFen, &u.UnfreezeGrantDay, &grantedFen, &lotsRaw, &locked, &skipUpline, &inviteRoot); err != nil {
			return err
		}
		u.InviteCode = invite.String
		u.UnfreezeGrantedFen = grantedFen
		u.Locked = locked != 0
		u.SkipUplineReward = skipUpline != 0
		u.InviteRoot = inviteRoot != 0
		d.users[u.ID] = u
		d.byWallet[strings.ToLower(u.WalletAddress)] = u.ID
		if u.InviteCode != "" {
			d.byInvite[u.InviteCode] = u.ID
		}
		d.balances[u.ID] = bal
		d.ispayLocked[u.ID] = lockedMicro
		d.ispayFree[u.ID] = freeMicro
		d.frozenUsdt[u.ID] = frozenUsdt
		d.ispayFrozen[u.ID] = frozenMicro
		d.unfreezeRemain[u.ID] = remainFen
		if len(lotsRaw) > 0 {
			var lots []frozenLot
			if err := json.Unmarshal(lotsRaw, &lots); err == nil {
				d.frozenLots[u.ID] = lots
			}
		}
	}

	prows, err := d.db.QueryContext(ctx, `SELECT id, name, description, price_fen, stock, image, category_id, status FROM products`)
	if err != nil {
		return err
	}
	defer prows.Close()
	for prows.Next() {
		p := &biz.Product{}
		var desc sql.NullString
		if err := prows.Scan(&p.ID, &p.Name, &desc, &p.PriceFen, &p.Stock, &p.Image, &p.CategoryID, &p.Status); err != nil {
			return err
		}
		p.Description = desc.String
		d.products[p.ID] = p
	}

	if d.packages == nil {
		d.packages = map[int64]*biz.Package{}
	}
	pkrows, err := d.db.QueryContext(ctx, `SELECT id, name, description, amount_fen, release_days, image, status, product_id, sort_order FROM packages`)
	if err != nil {
		return err
	}
	defer pkrows.Close()
	for pkrows.Next() {
		p := &biz.Package{}
		var desc sql.NullString
		if err := pkrows.Scan(&p.ID, &p.Name, &desc, &p.AmountFen, &p.ReleaseDays, &p.Image, &p.Status, &p.ProductID, &p.SortOrder); err != nil {
			return err
		}
		p.Description = desc.String
		d.packages[p.ID] = p
		if p.ID > d.packageSeq {
			d.packageSeq = p.ID
		}
	}

	orows, err := d.db.QueryContext(ctx, `SELECT id, user_id, status, total_fen, created_at, items_json, category_id, coins_micro, release_days, released_days FROM orders`)
	if err != nil {
		return err
	}
	defer orows.Close()
	for orows.Next() {
		o := &biz.Order{}
		var raw []byte
		if err := orows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalFen, &o.CreatedAt, &raw, &o.CategoryID, &o.CoinsMicro, &o.ReleaseDays, &o.ReleasedDays); err != nil {
			return err
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &o.Items)
		}
		d.orders[o.ID] = o
	}

	lrows, err := d.db.QueryContext(ctx, `SELECT id, user_id, amount_fen, balance_fen, type, ref_type, ref_id, created_at, tx_hash FROM ledger ORDER BY id`)
	if err != nil {
		return err
	}
	defer lrows.Close()
	for lrows.Next() {
		e := &biz.LedgerEntry{}
		var txHash sql.NullString
		if err := lrows.Scan(&e.ID, &e.UserID, &e.AmountFen, &e.BalanceFen, &e.Type, &e.RefType, &e.RefID, &e.CreatedAt, &txHash); err != nil {
			return err
		}
		if txHash.Valid {
			e.TxHash = txHash.String
		}
		d.ledger = append(d.ledger, e)
	}
	d.rebuildTxIndexLocked()
	d.seedFrozenLotsIfNeededLocked()
	return nil
}

func (d *Data) saveMySQL() error {
	ctx, cancel := context.WithTimeout(context.Background(), mysqlTimeout)
	defer cancel()
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, q := range []string{"DELETE FROM ledger", "DELETE FROM orders", "DELETE FROM packages", "DELETE FROM products", "DELETE FROM users", "DELETE FROM mall_meta"} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	st := d.settingsLocked()
	if _, err := tx.ExecContext(ctx, `INSERT INTO mall_meta (k, v) VALUES ('user_seq', ?), ('ledger_seq', ?), ('order_seq', ?), ('last_pair_settle_date', ?), ('ispay_price_fen', ?),
		('withdraw_fee_percent', ?), ('min_withdraw_fen', ?), ('max_recharge_fen', ?), ('pair_caps', ?), ('money_scale', ?)`,
		fmt.Sprintf("%d", d.userSeq), fmt.Sprintf("%d", d.ledgerSeq), fmt.Sprintf("%d", d.orderSeq), d.lastPairSettleDate, fmt.Sprintf("%d", st.IspayPriceFen),
		fmt.Sprintf("%d", st.WithdrawFeePercent), fmt.Sprintf("%d", st.MinWithdrawFen), fmt.Sprintf("%d", st.MaxRechargeFen),
		biz.EncodePairCapsJSON(st.PairCaps), fmt.Sprintf("%d", moneyScaleOrDefault(d.moneyScale))); err != nil {
		return err
	}
	for _, u := range d.users {
		if u == nil {
			continue
		}
		invite := any(u.InviteCode)
		if u.InviteCode == "" {
			invite = nil
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO users (id, wallet_address, nickname, avatar, invite_code, inviter_id, parent_id, side,
			left_id, right_id, chain_next_id, occupied_pos, perf_fen, settled_pair_fen, pair_cleared_right_fen, token_version,
			static_days, static_released_days, static_package_fen, dynamic_reward_day, dynamic_reward_fen, created_at, balance_fen,
			ispay_locked_micro, ispay_free_micro, frozen_usdt_fen, ispay_frozen_micro, unfreeze_remain_fen, unfreeze_grant_day, unfreeze_granted_fen, frozen_lots_json, locked, skip_upline_reward, invite_root)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			u.ID, u.WalletAddress, u.Nickname, u.Avatar, invite, u.InviterID, u.ParentID, u.Side,
			u.LeftID, u.RightID, u.ChainNextID, u.OccupiedPos, u.PerfFen, u.SettledPairFen, u.PairClearedRightFen, u.TokenVersion,
			u.StaticDays, u.StaticReleasedDays, u.StaticPackageFen, u.DynamicRewardDay, u.DynamicRewardFen, u.CreatedAt, d.balances[u.ID],
			d.ispayLocked[u.ID], d.ispayFree[u.ID], d.frozenUsdt[u.ID], d.ispayFrozen[u.ID], d.unfreezeRemain[u.ID], u.UnfreezeGrantDay, u.UnfreezeGrantedFen, lotsJSON(d.frozenLots[u.ID]), boolToTiny(u.Locked), boolToTiny(u.SkipUplineReward), boolToTiny(u.InviteRoot)); err != nil {
			return err
		}
	}
	for _, p := range d.products {
		if p == nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO products (id, name, description, price_fen, stock, image, category_id, status) VALUES (?,?,?,?,?,?,?,?)`,
			p.ID, p.Name, p.Description, p.PriceFen, p.Stock, p.Image, p.CategoryID, p.Status); err != nil {
			return err
		}
	}
	for _, p := range d.packages {
		if p == nil {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO packages (id, name, description, amount_fen, release_days, image, status, product_id, sort_order) VALUES (?,?,?,?,?,?,?,?,?)`,
			p.ID, p.Name, p.Description, p.AmountFen, p.ReleaseDays, p.Image, p.Status, p.ProductID, p.SortOrder); err != nil {
			return err
		}
	}
	for _, o := range d.orders {
		if o == nil {
			continue
		}
		raw, err := json.Marshal(o.Items)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO orders (id, user_id, status, total_fen, created_at, items_json, category_id, coins_micro, release_days, released_days) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			o.ID, o.UserID, o.Status, o.TotalFen, o.CreatedAt, raw, o.CategoryID, o.CoinsMicro, o.ReleaseDays, o.ReleasedDays); err != nil {
			return err
		}
	}
	for _, e := range d.ledger {
		if e == nil {
			continue
		}
		txHash := sql.NullString{String: e.TxHash, Valid: e.TxHash != ""}
		if _, err := tx.ExecContext(ctx, `INSERT INTO ledger (id, user_id, amount_fen, balance_fen, type, ref_type, ref_id, created_at, tx_hash) VALUES (?,?,?,?,?,?,?,?,?)`,
			e.ID, e.UserID, e.AmountFen, e.BalanceFen, e.Type, e.RefType, e.RefID, e.CreatedAt, txHash); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func lotsJSON(lots []frozenLot) any {
	if len(lots) == 0 {
		return nil
	}
	b, err := json.Marshal(lots)
	if err != nil {
		return nil
	}
	return string(b)
}

func atoi64(s string) int64 {
	var n int64
	_, _ = fmt.Sscan(s, &n)
	return n
}

func moneyScaleOrDefault(v int64) int64 {
	if v == biz.UstdScale {
		return v
	}
	if v <= 0 {
		return biz.UstdScale
	}
	return v
}

func boolToTiny(v bool) int64 {
	if v {
		return 1
	}
	return 0
}
