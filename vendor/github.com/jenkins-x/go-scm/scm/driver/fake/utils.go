package fake

func paginated(page, size, items int) (start, end int) {
	// handle the default value case for ListOptions.
	if page == 0 || size == 0 {
		start = 0
		end = items
		return
	}

	start = (page - 1) * size
	if start > items {
		start = items
	}
	end = start + size
	if end > items {
		end = items
	}
	return
}
