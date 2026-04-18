package stats

func GetRealtimeStats(minutes int) []map[string]interface{} {
	if DB == nil {
		return nil
	}
	rows, err := DB.Query(`
		SELECT
			DATE_TRUNC('minute', created_at) as minute,
			grp,
			COUNT(*) as count
		FROM stats
		WHERE created_at >= NOW() - ($1 || ' minutes')::INTERVAL
		GROUP BY minute, grp
		ORDER BY minute ASC
	`, minutes)
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := []map[string]interface{}{}
	for rows.Next() {
		var minute, grp string
		var count int
		rows.Scan(&minute, &grp, &count)
		result = append(result, map[string]interface{}{
			"minute": minute,
			"group":  grp,
			"count":  count,
		})
	}
	return result
}

func GetTodayByHour() []map[string]interface{} {
	if DB == nil {
		return nil
	}
	rows, err := DB.Query(`
		SELECT
			TO_CHAR(created_at AT TIME ZONE 'UTC', 'HH24:00') as hour,
			grp,
			COUNT(*) as count
		FROM stats
		WHERE created_at >= CURRENT_DATE
		GROUP BY hour, grp
		ORDER BY hour ASC
	`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	result := []map[string]interface{}{}
	for rows.Next() {
		var hour, grp string
		var count int
		rows.Scan(&hour, &grp, &count)
		result = append(result, map[string]interface{}{
			"hour":  hour,
			"group": grp,
			"count": count,
		})
	}
	return result
}
