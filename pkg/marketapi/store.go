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
		err = rows.Scan(&market.Id, &market.Description,
			&market.DateCreated, &market.DateClosed)
		if err != nil {
			return nil, err
		}
		markets = append(markets, market)
	}
	return markets, nil
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

	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	conn.ExecContext(ctx, "BEGIN EXCLUSIVE TRANSACTION;")
	defer conn.ExecContext(ctx, "ROLLBACK;")

	orderTime := now()

	rows, err := s.db.QueryContext(ctx, `
		SELECT uuid, shares_outstanding
		FROM securities
		WHERE market_id = ? 
		`, marketID)
	if err != nil {
		return err
	}

	allShares := []float64{}
	myIdx := -1
	rc := 0
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
	err = s.db.QueryRowContext(ctx, `
		SELECT tokens FROM portfolios WHERE user_id = ?`,
		userID).Scan(&heldTokens)
	if err != nil {
		return err
	}
	var heldSecurities float64
	if err := s.db.QueryRowContext(ctx, `
		SELECT amount FROM portfolio_securities 
		WHERE user_id = ? AND security_id = ?`,
		userID, securityUUID).Scan(&heldSecurities); err != nil {
		if err == sql.ErrNoRows {
			// this is ok, and not an error. ignore for now; we
			// simply don't own this security yet.
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
	_, err = s.db.ExecContext(ctx, `
		UPDATE portfolios 
		SET tokens = ?
		WHERE user_id = ?`, heldTokens-cost, userID)
	if err != nil {
		return err
	}
	// update held securities
	_, err = s.db.ExecContext(ctx, `
		UPDATE portfolio_securities 
		SET amount = ? 
		WHERE user_id = ? AND security_id = ?`,
		heldSecurities+amount, userID, securityID)
	if err != nil {
		return err
	}

	// add order to order book
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO orders (uuid, user_id, security_id, amount, cost, date)
		VALUES(?, ?, ?, ?, ?, ?)`,
		shortuuid.New(), userID, securityID, amount, cost, orderTime)
	if err != nil {
		return err
	}
	// calculate new price for this share.
	newPrice := lmsr.Price(lmsr.Liquidity, allShares[myIdx], allShares)
	// update security price log
	_, err = s.db.ExecContext(ctx, `
		UPDATE security_costs
		SET cost = ?, date = ?
		WHERE security_id = ?
	`, newPrice, orderTime, securityID)
	if err != nil {
		return err
	}
	// update shares_outstanding / last_price in securities table
	_, err = s.db.ExecContext(ctx, `
		UPDATE securities 
		SET shares_outstanding = ?, last_price = ?
		WHERE id = ?`, allShares[myIdx], newPrice, securityID)
	if err != nil {
		return err
	}
	// and commit the transaction. phew.
	_, err = conn.ExecContext(ctx, "COMMIT;")
	return err
}
