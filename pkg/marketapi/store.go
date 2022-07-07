package marketapi

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"

	"github.com/domino14/scrabfutures/pkg/lmsr"
	pb "github.com/domino14/scrabfutures/rpc/proto"
)

type SqliteStore struct {
	db *sql.DB
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

	marketID, err := s.dbid(ctx, "markets", "uuid", marketUUID)
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

	// 1. deduct `cost` tokens from user's portfolio
	//    - if we can't, fail
	//    - if cost is negative, we're selling - fail if we don't have this
	//      number of securities
	// 2. add order to order book
	// 3. calculate new price for this share (allShares has new breakdown)
	// 4. add to portfolio_securities
	// 5. update security_costs

	// meat

	conn.ExecContext(ctx, "COMMIT;")
}
