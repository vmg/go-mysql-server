package memory

import (
	"github.com/src-d/go-mysql-server/sql"
	"github.com/src-d/go-mysql-server/sql/expression"
)

type AscendIndexLookup struct {
	id    string
	Gte   []interface{}
	Lt    []interface{}
	Index ExpressionsIndex
}

var _ memoryIndexLookup = (*AscendIndexLookup)(nil)

func (l *AscendIndexLookup) ID() string { return l.id }

func (l *AscendIndexLookup) Values(p sql.Partition) (sql.IndexValueIter, error) {
	return &indexValIter{
		tbl:             l.Index.MemTable(),
		partition:       p,
		matchExpression: l.EvalExpression(),
	}, nil
}

func (l *AscendIndexLookup) Indexes() []string {
	return []string{l.id}
}

func (l *AscendIndexLookup) IsMergeable(lookup sql.IndexLookup) bool {
	_, ok := lookup.(MergeableLookup)
	return ok
}

func (l *AscendIndexLookup) Union(lookups ...sql.IndexLookup) sql.IndexLookup {
	return union(l.Index, l, lookups...)
}

func (l *AscendIndexLookup) EvalExpression() sql.Expression {
	if len(l.Index.ColumnExpressions()) > 1 {
		panic("Ascend index unsupported for multi-column indexes")
	}

	lt, typ := getType(l.Lt[0])
	ltexpr := expression.NewLessThan(l.Index.ColumnExpressions()[0], expression.NewLiteral(lt, typ))
	if len(l.Gte) > 0 {
		gte, _ := getType(l.Gte[0])
		return and(
			ltexpr,
			expression.NewGreaterThanOrEqual(l.Index.ColumnExpressions()[0], expression.NewLiteral(gte, typ)),
		)
	}
	return ltexpr
}

func (*AscendIndexLookup) Difference(...sql.IndexLookup) sql.IndexLookup {
	panic("ascendIndexLookup.Difference is not implemented")
}

func (l *AscendIndexLookup) Intersection(lookups ...sql.IndexLookup) sql.IndexLookup {
	return intersection(l.Index, l, lookups...)
}