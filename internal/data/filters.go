package data

import "strings"

type Filters struct {
	Page         int
	PageSize     int
	Sort         string
	SortSafeList []string
}

func (f *Filters) ValidateFilters(v *Validator) {
	v.Check(f.Page <= 10_000_000 && f.Page >= 1, "page", "page should be between 1 and 10,000,000")
	v.Check(f.PageSize <= 100 && f.PageSize >= 1, "page_size", "page size should be between 1 and 100")
	v.Check(In(f.Sort, f.SortSafeList...), "sort", "invalid sort value")
}

func (f Filters) SortColumn() string {
	for _, safeword := range f.SortSafeList {
		if f.Sort == safeword {
			return strings.TrimPrefix(f.Sort, "-")
		}
	}
	panic("unprocessable sort string: " + f.Sort)
}

func (f Filters) SortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}
	return "ASC"
}

func (f Filters) limit() int {
	return f.PageSize
}

func (f Filters) offset() int {
	return (f.Page - 1) * f.PageSize
}
