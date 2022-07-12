package marketapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lithammer/shortuuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"

	"github.com/domino14/scrabfutures/pkg/lmsr"
	pb "github.com/domino14/scrabfutures/rpc/proto"
)

type SqliteStore struct {
	db *sql.DB
}

func now() string {
	return time.Now().Format(time.RFC3339)
}

func NewSqliteStore(dbName string) (*SqliteStore, error) {
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		return nil, err
	}
	return &SqliteStore{db: db}, nil
}

func (s *SqliteStore) dbid(ctx context.Context, tableName, otheridName, otherid string) (int64, error) {
	var dbid int64

	query := fmt.Sprintf("SELECT id FROM %s WHERE %s = ?", tableName, otheridName)

	err := s.db.QueryRowContext(ctx, query, otherid).Scan(&dbid)
	if err != nil {
		return 0, err
	}
	return dbid, nil
}

func (s *SqliteStore) GetOrderBook(ctx context.Context, marketID string, securityID string,
	username string, sinceDate time.Time, limit int) ([]*pb.Order, error) {

	wheres := []string{}
	wheresVars := []any{}

	if username != "" {
		dbid, err := s.dbid(ctx, "users", "username", username)
		if err != nil {
			return nil, err
		}
		wheres = append(wheres, `user_id = ?`)
		wheresVars = append(wheresVars, dbid)
	}

	if marketID != "" {
		dbid, err := s.dbid(ctx, "markets", "uuid", marketID)
		if err != nil {
			return nil, err
		}
		wheres = append(wheres, `market_id = ?`)
		wheresVars = append(wheresVars, dbid)
	}

	if securityID != "" {
		dbid, err := s.dbid(ctx, "securities", "uuid", securityID)
		if err != nil {
			return nil, err
		}
		wheres = append(wheres, `security_id = ?`)
		wheresVars = append(wheresVars, dbid)
	}

	wheres = append(wheres, `sinceDate >= ?`)
	wheresVars = append(wheresVars, sinceDate)

	whereRendered := strings.Join(wheres, " AND ")
	limitRendered := ""
	if limit > 0 {
		limitRendered = fmt.Sprintf("LIMIT %d", limit)
	}
	fullQuery := fmt.Sprintf(`
		SELECT uuid, securities.uuid, securities.shortname,
		amount, cost, date_created
		FROM orders
		JOIN securities
		ON orders.security_id = securities.id
		%s
		%s	
	`, whereRendered, limitRendered)
	log.Debug().Str("fullQuery", fullQuery).Str("storeMethod", "GetOrderBook").Msg("executing-query")
	rows, err := s.db.QueryContext(ctx, fullQuery, wheresVars...)
	if err != nil {
		return nil, err
	}
	orders := []*pb.Order{}
	defer rows.Close()
	for rows.Next() {
		order := &pb.Order{}
		err = rows.Scan(&order.Id, &order.SecurityId, &order.SecurityShortname, &order.Amount, &order.Cost, &order.DateCreated)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func (s *SqliteStore) GetMarket(ctx context.Context, id string) (*pb.Market, error) {
	market := &pb.Market{}
	var dateClosed sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT description, date_created, is_open, date_closed
		FROM markets
		WHERE uuid = ?`, id).Scan(
		&market.Description, &market.DateCreated, &market.IsOpen, &dateClosed)
	if err != nil {
		return nil, err
	}
	market.Id = id
	market.DateClosed = dateClosed.String
	return market, nil
}

func (s *SqliteStore) GetOpenMarkets(ctx context.Context) ([]*pb.Market, error) {

	rows, err := s.db.QueryContext(ctx, `
		SELECT uuid, description, date_created, date_closed 
		FROM markets
		WHERE is_open = 1`)

	if err != nil {
		return nil, err
	}

	markets := []*pb.Market{}
	defer rows.Close()

	for rows.Next() {
		market := &pb.Market{}
		var dateClosed sql.NullString
		err = rows.Scan(&market.Id, &market.Description,
			&market.DateCreated, &dateClosed)
		if err != nil {
			return nil, err
		}
		market.DateClosed = dateClosed.String // can be empty, that's ok.
		market.IsOpen = true
		markets = append(markets, market)
	}
	return markets, nil
}

func (s *SqliteStore) CreateMarket(ctx context.Context, description string) (string, error) {
	id := shortuuid.New()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO markets(uuid, description, date_created, is_open)
		values(?, ?, ?, ?)
	`, id, description, now(), 0)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (s *SqliteStore) OpenMarket(ctx context.Context, uuid string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE markets SET is_open = 1 WHERE uuid = ?
	`, uuid)
	return err
}

func (s *SqliteStore) CloseMarket(ctx context.Context, uuid string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE markets SET is_open = 0, date_closed = ? WHERE uuid = ?
	`, now(), uuid)
	return err
}

func (s *SqliteStore) DeleteMarket(ctx context.Context, uuid string) error {
	m, err := s.GetMarket(ctx, uuid)
	if err != nil {
		return err
	}
	if m.IsOpen || m.DateClosed != "" {
		// if this market was ever opened, then we cannot delete it.
		return errors.New("disallowed deletion of market that was once open")
	}
	_, err = s.db.ExecContext(ctx, `DELETE FROM markets WHERE uuid = ?`, uuid)
	return err
}

// AddSecurities adds one or more securities to a market. Securities cannot
// be added once a market is opened.
func (s *SqliteStore) AddSecurities(ctx context.Context, marketID string,
	securities []*pb.AddSecuritiesRequest_Security) error {

	m, err := s.GetMarket(ctx, marketID)
	if err != nil {
		return err
	}
	if m.IsOpen || m.DateClosed != "" {
		return errors.New("disallowed adding of securities to market that was once open")
	}

	mdbid, err := s.dbid(ctx, "markets", "uuid", marketID)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	addDate := now()

	for _, sec := range securities {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO securities(uuid, description, shortname, date_created,
				market_id, shares_outstanding)
			VALUES (?, ?, ?, ?, ?, ?)
		`, shortuuid.New(), sec.Description, sec.Shortname, addDate, mdbid, 0.0)
		if err != nil {
			return err
		}
	}

	// Now edit all the prices...
	err = s.editAllSecurityPrices(ctx, tx, mdbid)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

// DeleteSecurity deletes a security from a market. Securities cannot
// be deleted once a market is opened.
func (s *SqliteStore) DeleteSecurity(ctx context.Context, marketID string,
	securityID string) error {

	m, err := s.GetMarket(ctx, marketID)
	if err != nil {
		return err
	}
	if m.IsOpen || m.DateClosed != "" {
		return errors.New("disallowed deletion of securities from market that was once open")
	}

	mdbid, err := s.dbid(ctx, "markets", "uuid", marketID)
	if err != nil {
		return err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `DELETE FROM securities WHERE uuid = ?`, securityID)
	if err != nil {
		return err
	}

	// Now edit all the prices...
	err = s.editAllSecurityPrices(ctx, tx, mdbid)
	if err != nil {
		return err
	}
	err = tx.Commit()
	if err != nil {
		return err
	}
	return nil
}

func (s *SqliteStore) editAllSecurityPrices(ctx context.Context, tx *sql.Tx, marketDBID int64) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT uuid, shares_outstanding
		FROM securities
		WHERE market_id = ? 
		`, marketDBID)
	if err != nil {
		return err
	}

	allShares := []float64{}
	allShareUUIDs := []string{}
	defer rows.Close()
	for rows.Next() {
		var shares float64
		var uuid string
		err = rows.Scan(&uuid, &shares)
		if err != nil {
			return err
		}
		allShares = append(allShares, shares)
		allShareUUIDs = append(allShareUUIDs, uuid)
	}

	// calculate new price for all shares in this market.
	for idx := range allShares {
		np := lmsr.Price(lmsr.Liquidity, allShares, idx)
		_, err = tx.ExecContext(ctx, `
			UPDATE securities 
			SET last_price = ?
			WHERE uuid = ?`, np, allShareUUIDs[idx])
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *SqliteStore) GetSecurity(ctx context.Context, uuid string) (*pb.Security, error) {
	security := &pb.Security{}
	err := s.db.QueryRowContext(ctx, `
		SELECT securities.description, shortname, securities.date_created, 
			markets.uuid, shares_outstanding,last_price
		FROM securities
		JOIN markets 
		ON securities.market_id = markets.id
		WHERE securities.uuid = ?`, uuid).Scan(
		&security.Description, &security.Shortname, &security.DateCreated,
		&security.MarketId, &security.SharesOutstanding, &security.LastPrice)
	if err != nil {
		return nil, err
	}
	security.Id = uuid
	return security, nil
}

func (s *SqliteStore) GetSecurities(ctx context.Context, marketID string) ([]*pb.Security, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT securities.uuid, securities.description, securities.shortname, 
			securities.date_created, shares_outstanding,
			last_price
		FROM securities
		JOIN markets ON securities.market_id = markets.id
		WHERE markets.uuid = ?
		`, marketID)
	if err != nil {
		return nil, err
	}

	securities := []*pb.Security{}
	defer rows.Close()
	for rows.Next() {
		security := &pb.Security{}
		err = rows.Scan(&security.Id, &security.Description, &security.Shortname,
			&security.DateCreated, &security.SharesOutstanding, &security.LastPrice)
		if err != nil {
			return nil, err
		}
		securities = append(securities, security)
	}
	return securities, nil
}

func (s *SqliteStore) FulfillOrder(ctx context.Context, username string,
	securityUUID, marketUUID string, amount float64, buy bool) error {
	// this function is too long. simplify.
	if amount <= 0 {
		return errors.New("amount must be positive")
	}
	marketID, err := s.dbid(ctx, "markets", "uuid", marketUUID)
	if err != nil {
		return err
	}
	userID, err := s.dbid(ctx, "users", "username", username)
	if err != nil {
		return err
	}
	securityID, err := s.dbid(ctx, "securities", "uuid", securityUUID)
	if err != nil {
		return err
	}

	m, err := s.GetMarket(ctx, marketUUID)
	if err != nil {
		return err
	}
	if !m.IsOpen {
		return errors.New("this market is closed")
	}

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	conn.ExecContext(ctx, "BEGIN EXCLUSIVE TRANSACTION;")
	defer conn.ExecContext(ctx, "ROLLBACK;")

	orderTime := now()

	rows, err := conn.QueryContext(ctx, `
		SELECT uuid, shares_outstanding
		FROM securities
		WHERE market_id = ? 
		`, marketID)
	if err != nil {
		return err
	}

	allShares := []float64{}
	allShareUUIDs := []string{}
	myIdx := -1
	rc := 0
	defer rows.Close()
	for rows.Next() {
		var shares float64
		var uuid string
		err = rows.Scan(&uuid, &shares)
		if err != nil {
			return err
		}
		if uuid == securityUUID {
			myIdx = rc
		}
		allShares = append(allShares, shares)
		allShareUUIDs = append(allShareUUIDs, uuid)
		rc += 1
	}
	if myIdx == -1 {
		// We never found the security index.
		return errors.New("securityUUID not found")
	}
	if !buy {
		amount *= -1
	}

	cost := lmsr.TradeCost(lmsr.Liquidity, amount, allShares, myIdx)
	var heldTokens float64
	err = conn.QueryRowContext(ctx, `
		SELECT tokens FROM portfolios WHERE user_id = ?`,
		userID).Scan(&heldTokens)
	if err != nil {
		return err
	}
	var heldSecurities float64
	alreadyOwned := true
	if err := conn.QueryRowContext(ctx, `
		SELECT amount FROM portfolio_securities 
		WHERE user_id = ? AND security_id = ?`,
		userID, securityID).Scan(&heldSecurities); err != nil {
		if err == sql.ErrNoRows {
			// this is ok, and not an error. ignore for now; we
			// simply don't own this security yet.
			alreadyOwned = false
		} else {
			return err
		}
	}

	if cost > 0 {
		if amount < 0 {
			return errors.New("unexpected amount - negative")
		}
		if heldTokens < cost {
			return errors.New("not enough tokens for this transaction")
		}

	} else if cost < 0 {
		if amount > 0 {
			return errors.New("unexpected amount - positive")
		}
		if heldSecurities < -amount {
			return errors.New("cannot sell more securities than we own")
		}

	}

	// update tokens
	_, err = conn.ExecContext(ctx, `
		UPDATE portfolios 
		SET tokens = ?
		WHERE user_id = ?`, heldTokens-cost, userID)
	if err != nil {
		return err
	}
	// update held securities
	if alreadyOwned {
		_, err = conn.ExecContext(ctx, `
		UPDATE portfolio_securities 
		SET amount = ? 
		WHERE user_id = ? AND security_id = ?`,
			heldSecurities+amount, userID, securityID)
		if err != nil {
			return err
		}
	} else {
		_, err = conn.ExecContext(ctx, `
		INSERT INTO portfolio_securities(amount, user_id, security_id)
		VALUES(?, ?, ?)
	`, amount, userID, securityID)
		if err != nil {
			return err
		}
	}

	// add order to order book
	_, err = conn.ExecContext(ctx, `
		INSERT INTO orders (uuid, user_id, security_id, amount, cost, date)
		VALUES(?, ?, ?, ?, ?, ?)`,
		shortuuid.New(), userID, securityID, amount, cost, orderTime)
	if err != nil {
		return err
	}
	// calculate new price for all shares in this market.
	for idx := range allShares {
		np := lmsr.Price(lmsr.Liquidity, allShares, idx)
		// update security price log
		_, err = conn.ExecContext(ctx, `
			INSERT INTO security_costs(security_id, cost, date)
			VALUES(?, ?, ?)
			`, allShareUUIDs[idx], np, orderTime)
		if err != nil {
			return err
		}

		_, err = conn.ExecContext(ctx, `
			UPDATE securities 
			SET shares_outstanding = ?, last_price = ?
			WHERE uuid = ?`, allShares[idx], np, allShareUUIDs[idx])
		if err != nil {
			return err
		}

	}

	// and commit the transaction. phew.
	_, err = conn.ExecContext(ctx, "COMMIT;")

	return err
}
