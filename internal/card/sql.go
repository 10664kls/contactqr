package card

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/10664kls/contactqr/internal/pager"
	sq "github.com/Masterminds/squirrel"
)

var ErrCardNotFound = errors.New("card not found")

type CardQuery struct {
	ID            string    `json:"id" param:"id"`
	EmployeeID    string    `json:"employeeId" param:"employeeId"`
	DisplayName   string    `json:"displayName" param:"displayName"`
	JobTitle      string    `json:"jobTitle" param:"jobTitle"`
	Department    string    `json:"department" param:"department"`
	Company       string    `json:"company" param:"company"`
	Status        string    `json:"status" param:"status"`
	CreatedAfter  time.Time `json:"createdAfter" param:"createdAfter"`
	CreatedBefore time.Time `json:"createdBefore" param:"createdBefore"`
	PageToken     string    `json:"pageToken" param:"pageToken"`
	PageSize      uint64    `json:"pageSize" param:"pageSize"`
}

func (q *CardQuery) ToSql() (string, []any, error) {
	and := sq.And{}

	if q.ID != "" {
		and = append(and, sq.Eq{"id": q.ID})
	}

	if q.EmployeeID != "" {
		and = append(and, sq.Eq{"employee_id": q.EmployeeID})
	}

	if q.DisplayName != "" {
		and = append(and, sq.Expr("display_name LIKE ?", "%"+q.DisplayName+"%"))
	}

	if q.JobTitle != "" {
		and = append(and, sq.Eq{"job_title": q.JobTitle})
	}

	if q.Department != "" {
		and = append(and, sq.Eq{"department": q.Department})
	}

	if q.Company != "" {
		and = append(and, sq.Eq{"company": q.Company})
	}

	if q.Status != "" {
		and = append(and, sq.Eq{"status": q.Status})
	}

	if !q.CreatedBefore.IsZero() {
		and = append(and, sq.LtOrEq{"created_at": q.CreatedBefore})
	}
	if !q.CreatedAfter.IsZero() {
		and = append(and, sq.GtOrEq{"created_at": q.CreatedAfter})
	}

	if q.PageToken != "" {
		cursor, err := pager.DecodeCursor(q.PageToken)
		if err != nil {
			return "", nil, err
		}
		and = append(and, sq.Expr("(created_at, id) < (?, ?)", cursor.ID))
	}

	return and.ToSql()
}

func listCards(ctx context.Context, db *sql.DB, in *CardQuery) ([]*Card, error) {
	id := fmt.Sprintf("TOP %d id", pager.Size(in.PageSize))
	pred, args, err := in.ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	q, args := sq.
		Select(
			id,
			"employee_id",
			"display_name",
			"job_title",
			"department",
			"company",
			"email_address",
			"phone_number",
			"mobile_number",
			"status",
			"remark",
			"created_at",
			"updated_at",
			"created_by",
			"updated_by",
		).
		From("dbo.business_card").
		Where(pred, args...).
		OrderBy("created_at DESC").
		PlaceholderFormat(sq.AtP).
		MustSql()

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	cards := make([]*Card, 0)
	for rows.Next() {
		var c Card
		if err := rows.Scan(
			&c.ID,
			&c.EmployeeID,
			&c.DisplayName,
			&c.JobTitle,
			&c.Department,
			&c.Company,
			&c.Email,
			&c.PhoneNumber,
			&c.MobileNumber,
			&c.Status,
			&c.Remark,
			&c.CreatedAt,
			&c.UpdatedAt,
			&c.createdBy,
			&c.updatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		cards = append(cards, &c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate rows: %w", err)
	}

	return cards, nil
}

func getCard(ctx context.Context, db *sql.DB, in *CardQuery) (*Card, error) {
	in.PageSize = 1
	if in.ID == "" {
		return nil, ErrCardNotFound
	}

	cards, err := listCards(ctx, db, in)
	if err != nil {
		return nil, err
	}

	if len(cards) == 0 {
		return nil, ErrCardNotFound
	}

	return cards[0], nil
}

func createCard(ctx context.Context, db *sql.DB, in *Card) error {
	q, args := sq.
		Insert("dbo.business_card").
		Columns(
			"id",
			"employee_id",
			"display_name",
			"job_title",
			"department",
			"company",
			"email_address",
			"phone_number",
			"mobile_number",
			"status",
			"remark",
			"created_at",
			"updated_at",
			"created_by",
			"updated_by",
		).
		Values(
			in.ID,
			in.EmployeeID,
			in.DisplayName,
			in.JobTitle,
			in.Department,
			in.Company,
			in.Email,
			in.PhoneNumber,
			in.MobileNumber,
			in.Status,
			in.Remark,
			in.CreatedAt,
			in.UpdatedAt,
			in.createdBy,
			in.updatedBy,
		).
		PlaceholderFormat(sq.AtP).
		MustSql()

	if _, err := db.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

func updateCard(ctx context.Context, db *sql.DB, in *Card) error {
	q, args := sq.
		Update("dbo.business_card").
		Set("display_name", in.DisplayName).
		Set("job_title", in.JobTitle).
		Set("department", in.Department).
		Set("company", in.Company).
		Set("email_address", in.Email).
		Set("phone_number", in.PhoneNumber).
		Set("mobile_number", in.MobileNumber).
		Set("status", in.Status).
		Set("remark", in.Remark).
		Set("updated_at", in.UpdatedAt).
		Set("updated_by", in.updatedBy).
		Where(
			sq.Eq{
				"id": in.ID,
			}).
		PlaceholderFormat(sq.AtP).
		MustSql()

	if _, err := db.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}
