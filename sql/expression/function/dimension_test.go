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

func TestDimension(t *testing.T) {
	t.Run("point is dimension 0", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(sql.Point{X: 1, Y: 2}, sql.PointType{}))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(0, v)
	})

	t.Run("linestring is dimension 1", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(sql.LineString{Points: []sql.Point{{X: 0, Y: 1}, {X: 2, Y: 3}}}, sql.LineStringType{}))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(1, v)
	})

	t.Run("polygon dimension 2", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(sql.Polygon{Lines: []sql.LineString{{Points: []sql.Point{{X: 0, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}, {X: 0, Y: 0}}}}}, sql.PolygonType{}))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(2, v)
	})

	t.Run("multipoint is dimension 0", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(sql.MultiPoint{Points: []sql.Point{{X: 0, Y: 1}, {X: 2, Y: 3}}}, sql.MultiPointType{}))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(0, v)
	})

	t.Run("multilinestring is dimension 1", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(sql.MultiLineString{Lines: []sql.LineString{{Points: []sql.Point{{X: 0, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}, {X: 0, Y: 0}}}}}, sql.MultiLineStringType{}))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(1, v)
	})

	t.Run("geometry with inner point is dimension 0", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(sql.Point{X: 1, Y: 2}, sql.GeometryType{}))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(0, v)
	})

	t.Run("geometry with inner linestring is dimension 1", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(sql.LineString{Points: []sql.Point{{X: 0, Y: 1}, {X: 2, Y: 3}}}, sql.GeometryType{}))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(1, v)
	})

	t.Run("geometry with inner polygon dimension 2", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(sql.Polygon{Lines: []sql.LineString{{Points: []sql.Point{{X: 0, Y: 0}, {X: 1, Y: 1}, {X: 0, Y: 1}, {X: 0, Y: 0}}}}}, sql.GeometryType{}))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(2, v)
	})

	t.Run("empty geometry collection has dimension null", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(sql.GeomColl{}, sql.GeometryType{}))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(nil, v)
	})

	t.Run("geometry collection of a point has dimension 0", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(sql.GeomColl{Geoms: []sql.GeometryValue{sql.Point{}}}, sql.GeometryType{}))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(0, v)
	})

	t.Run("geometry collection of a different types takes highest type", func(t *testing.T) {
		require := require.New(t)
		point := sql.Point{}
		line := sql.LineStringType{}.Zero().(sql.LineString)
		poly := sql.PolygonType{}.Zero().(sql.Polygon)
		f := NewDimension(expression.NewLiteral(sql.GeomColl{Geoms: []sql.GeometryValue{point, line, poly}}, sql.GeometryType{}))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(2, v)
	})

	t.Run("geometry collection null is the largest dimension", func(t *testing.T) {
		require := require.New(t)
		point := sql.Point{}
		line := sql.LineStringType{}.Zero().(sql.LineString)
		poly := sql.PolygonType{}.Zero().(sql.Polygon)
		geom := sql.GeomColl{}
		f := NewDimension(expression.NewLiteral(sql.GeomColl{Geoms: []sql.GeometryValue{point, line, poly, geom}}, sql.GeometryType{}))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(nil, v)
	})

	t.Run("null is null", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(123, sql.Int64))
		_, err := f.Eval(sql.NewEmptyContext(), nil)
		require.Error(err)
	})

	t.Run("null is null", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(nil, sql.Null))
		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)
		require.Equal(nil, v)
	})

	t.Run("check return type", func(t *testing.T) {
		require := require.New(t)
		f := NewDimension(expression.NewLiteral(sql.Point{X: 1, Y: 2}, sql.PointType{}))

		v, err := f.Eval(sql.NewEmptyContext(), nil)
		require.NoError(err)

		typ := f.Type()
		_, err = typ.Convert(v)
		require.NoError(err)
	})
}
