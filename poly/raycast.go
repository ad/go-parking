// Raycast shows an implementation of the ray casting point-in-polygon
// (PNPoly) algorithm for testing if a point is inside a closed polygon.
// Also known as the crossing number or the even-odd rule algorithm.
//
// The implementation follows
// https://www.ecse.rpi.edu/Homepages/wrf/Research/Short_Notes/pnpoly.html
package poly

// XY is a 2D point in the Cartesian plane.
type XY struct {
	X, Y float64
}

// Poly represents a closed polygon.  Pairs of consecutive points represent
// endpoints of segments.  The last and first point represent an additional
// segment.  That is, the last point does not need to repeat the first to
// close the polygon.
type Poly struct {
	XY            []XY
	Zero, NonZero int
}

func (pt *Poly) IncZero() {
	pt.Zero++
}

func (pt *Poly) IncNonZero() {
	pt.NonZero++
}

// In returns true if pt is inside pg.
//
// Segments of the polygon are allowed to cross.  In this case they divide the
// polygon into multiple regions.  The function returns true for points in
// regions on the perimeter of the polygon.  The return value for interior
// regions is determined by a two coloring of the regions.
//
// If pt is exactly on a segment or vertex of pg, the method may return true or
// false.
func (pt XY) In(pg Poly) bool {
	if len(pg.XY) < 3 {
		return false
	}
	a := pg.XY[0]
	in := rayIntersectsSegment(pt, pg.XY[len(pg.XY)-1], a)
	for _, b := range pg.XY[1:] {
		if rayIntersectsSegment(pt, a, b) {
			in = !in
		}
		a = b
	}
	return in
}

// Segment intersect expression from
// https://www.ecse.rpi.edu/Homepages/wrf/Research/Short_Notes/pnpoly.html
//
// Currently the compiler inlines the function by default.
func rayIntersectsSegment(p, a, b XY) bool {
	return (a.Y > p.Y) != (b.Y > p.Y) &&
		p.X < (b.X-a.X)*(p.Y-a.Y)/(b.Y-a.Y)+a.X
}

func (pt *Poly) Center() XY {
	vertices := pt.XY
	vertexCount := len(vertices)

	centroid := XY{0, 0}
	signedArea := 0.0
	x0 := 0.0
	y0 := 0.0
	x1 := 0.0
	y1 := 0.0
	a := 0.0

	// For all vertices except last
	i := 0
	for i < vertexCount-1 {
		x0 = vertices[i].X
		y0 = vertices[i].Y
		x1 = vertices[i+1].X
		y1 = vertices[i+1].Y
		a = x0*y1 - x1*y0
		signedArea += a
		centroid.X += (x0 + x1) * a
		centroid.Y += (y0 + y1) * a
		i++
	}

	// Do last vertex separately to avoid performing an expensive modulus operation in each iteration.
	x0 = vertices[i].X
	y0 = vertices[i].Y
	x1 = vertices[0].X
	y1 = vertices[0].Y
	a = x0*y1 - x1*y0
	signedArea += a
	centroid.X += (x0 + x1) * a
	centroid.Y += (y0 + y1) * a

	signedArea *= 0.5
	centroid.X /= (6.0 * signedArea)
	centroid.Y /= (6.0 * signedArea)

	return centroid
}

func (pt *Poly) MinMax() (min, max XY) {
	min = pt.XY[0]
	max = pt.XY[0]

	for _, v := range pt.XY {
		if v.X < min.X {
			min.X = v.X
		}
		if v.Y < min.Y {
			min.Y = v.Y
		}
		if v.X > max.X {
			max.X = v.X
		}
		if v.Y > max.Y {
			max.Y = v.Y
		}
	}

	return
}

func MinMaxMany(polys []*Poly) (min, max XY) {
	min = polys[0].XY[0]
	max = polys[0].XY[0]

	for _, pt := range polys {
		for _, v := range pt.XY {
			if v.X < min.X {
				min.X = v.X
			}
			if v.Y < min.Y {
				min.Y = v.Y
			}
			if v.X > max.X {
				max.X = v.X
			}
			if v.Y > max.Y {
				max.Y = v.Y
			}
		}
	}

	return
}
