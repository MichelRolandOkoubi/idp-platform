package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/MichelRolandOkoubi/idp-platform/control-plane/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	pool *pgxpool.Pool
}

func NewPostgres(url string) (*DB, error) {
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{pool: pool}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) Migrate() error {
	_, err := db.pool.Exec(context.Background(), migrations)
	return err
}

const migrations = `
CREATE TABLE IF NOT EXISTS users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username    VARCHAR(255) UNIQUE NOT NULL,
    email       VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    roles       TEXT[] DEFAULT '{"developer"}',
    created_at  TIMESTAMPTZ DEFAULT NOW(),
    updated_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cost_history (
    id          BIGSERIAL PRIMARY KEY,
    namespace   VARCHAR(255) NOT NULL,
    app_name    VARCHAR(255),
    amount      DECIMAL(10,4) NOT NULL,
    currency    VARCHAR(10) DEFAULT 'USD',
    recorded_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS anomalies (
    id            BIGSERIAL PRIMARY KEY,
    namespace     VARCHAR(255) NOT NULL,
    severity      VARCHAR(50) NOT NULL,
    message       TEXT NOT NULL,
    current_value DECIMAL(10,4),
    expected_value DECIMAL(10,4),
    z_score       DECIMAL(10,4),
    detected_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cost_history_namespace ON cost_history(namespace);
CREATE INDEX IF NOT EXISTS idx_cost_history_recorded ON cost_history(recorded_at);
CREATE INDEX IF NOT EXISTS idx_anomalies_namespace ON anomalies(namespace);
`

type CostDataPoint struct {
	Date     string  `json:"date"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	AppName  string  `json:"app_name,omitempty"`
}

type Anomaly struct {
	DetectedAt    string  `json:"detected_at"`
	Severity      string  `json:"severity"`
	Message       string  `json:"message"`
	CurrentValue  float64 `json:"current_value"`
	ExpectedValue float64 `json:"expected_value"`
	ZScore        float64 `json:"z_score"`
}

func (db *DB) GetCostHistory(ctx context.Context, namespace string, days int) ([]*CostDataPoint, error) {
	rows, err := db.pool.Query(ctx, `
        SELECT
            DATE(recorded_at) as date,
            SUM(amount) as amount,
            currency
        FROM cost_history
        WHERE namespace = $1
          AND recorded_at >= NOW() - INTERVAL '1 day' * $2
        GROUP BY date, currency
        ORDER BY date DESC
    `, namespace, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []*CostDataPoint
	for rows.Next() {
		p := &CostDataPoint{}
		if err := rows.Scan(&p.Date, &p.Amount, &p.Currency); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, nil
}

func (db *DB) GetAnomalies(ctx context.Context, namespace string) ([]*Anomaly, error) {
	rows, err := db.pool.Query(ctx, `
        SELECT detected_at, severity, message, current_value, expected_value, z_score
        FROM anomalies
        WHERE namespace = $1
        ORDER BY detected_at DESC
        LIMIT 50
    `, namespace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anomalies []*Anomaly
	for rows.Next() {
		a := &Anomaly{}
		var detectedAt time.Time
		if err := rows.Scan(
			&detectedAt, &a.Severity, &a.Message,
			&a.CurrentValue, &a.ExpectedValue, &a.ZScore,
		); err != nil {
			return nil, err
		}
		a.DetectedAt = detectedAt.Format(time.RFC3339)
		anomalies = append(anomalies, a)
	}
	return anomalies, nil
}

// Implement DBInterface for auth.Service
func (db *DB) GetUserByUsername(username string) (*auth.User, error) {
	u := &auth.User{}
	err := db.pool.QueryRow(context.Background(), `
        SELECT id, username, email, password_hash, roles
        FROM users WHERE username = $1
    `, username).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.Roles)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (db *DB) CreateUser(u *auth.User) error {
	_, err := db.pool.Exec(context.Background(), `
        INSERT INTO users (username, email, password_hash, roles)
        VALUES ($1, $2, $3, $4)
    `, u.Username, u.Email, u.PasswordHash, u.Roles)
	return err
}
