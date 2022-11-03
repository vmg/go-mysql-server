// Copyright 2020-2021 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package function

import (
	"testing"

	"github.com/stretchr/testify/require"

	"vitess.io/vitess/go/test/go-mysql-server/sql"
	"vitess.io/vitess/go/test/go-mysql-server/sql/expression"
)

func TestConcat(t *testing.T) {
	t.Run("concat multiple arguments", func(t *testing.T) {
		require := require.New(t)
		f, err := NewConcat(expression.NewLiteral("foo", sql.LongText),
			expression.NewLiteral(5, sql.LongText),
			expression.NewLiteral(true, sql.Boolean),
		)
		require.NoError(err)

		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal("foo5true", v)
	})

	t.Run("some argument is nil", func(t *testing.T) {
		require := require.New(t)
		f, err := NewConcat(expression.NewLiteral("foo", sql.LongText),
			expression.NewLiteral(nil, sql.LongText),
			expression.NewLiteral(true, sql.Boolean),
		)
		require.NoError(err)

		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(nil, v)
	})
}

func TestNewConcat(t *testing.T) {
	require := require.New(t)

	_, err := NewConcat(expression.NewLiteral(nil, sql.LongText))
	require.NoError(err)

	_, err = NewConcat(expression.NewLiteral(nil, sql.LongText), expression.NewLiteral(nil, sql.Int64))
	require.NoError(err)

	_, err = NewConcat(
		expression.NewLiteral(nil, sql.LongText),
		expression.NewLiteral(nil, sql.Boolean),
		expression.NewLiteral(nil, sql.Int64),
		expression.NewLiteral(nil, sql.LongText),
	)
	require.NoError(err)
}
