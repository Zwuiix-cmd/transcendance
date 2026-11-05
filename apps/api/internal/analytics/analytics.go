package analytics

import (
	"math"

	"card-arena/api/internal/model"
)

const RecentActivityLimit = 30

// SummarizeActivity computes the values displayed beside a player's activity
// chart. The API owns these aggregates so every client sees the same numbers.
func SummarizeActivity(points []model.UserActivityPoint) model.ActivityMetrics {
	metrics := model.ActivityMetrics{}
	for index, point := range points {
		games := positiveInt(point.Games)
		metrics.TotalGames += games
		metrics.TotalWins += positiveInt(point.Wins)
		metrics.TotalLosses += positiveInt(point.Losses)
		metrics.TotalScore += point.Score
		metrics.TotalEloDelta += point.EloDelta
		if games > 0 {
			metrics.ActiveDays++
		}
		if games > metrics.BestDayGames {
			metrics.BestDay = index
			metrics.BestDayGames = games
		}
	}
	if metrics.TotalGames > 0 {
		metrics.WinRate = float64(metrics.TotalWins) / float64(metrics.TotalGames)
	}
	if metrics.ActiveDays > 0 {
		metrics.AverageGamesPerActiveDay = float64(metrics.TotalGames) / float64(metrics.ActiveDays)
	}
	return metrics
}

func PrepareEloChart(points []model.EloHistoryPoint) *model.EloChartGeometry {
	if len(points) == 0 {
		return nil
	}

	minimum := math.Inf(1)
	maximum := math.Inf(-1)
	for _, point := range points {
		minimum = math.Min(minimum, float64(point.PreviousElo))
		minimum = math.Min(minimum, float64(point.NewElo))
		maximum = math.Max(maximum, float64(point.PreviousElo))
		maximum = math.Max(maximum, float64(point.NewElo))
	}
	padding := math.Max(20, (maximum-minimum)*0.14)
	minimum = math.Max(0, minimum-padding)
	maximum += padding
	rangeSize := math.Max(1, maximum-minimum)

	geometry := &model.EloChartGeometry{
		Min:    minimum,
		Max:    maximum,
		Points: make([][2]float64, len(points)),
	}
	for index, point := range points {
		geometry.Points[index] = [2]float64{
			chartCoordinate(index, len(points)),
			math.Min(1, math.Max(0, (float64(point.NewElo)-minimum)/rangeSize)),
		}
	}
	return geometry
}

func PreparePlayerActivityChart(points []model.UserActivityPoint) *model.PlayerActivityChartGeometry {
	if len(points) == 0 {
		return nil
	}

	maximum := 1.0
	for _, point := range points {
		maximum = math.Max(maximum, float64(positiveInt(point.Games)))
	}
	geometry := &model.PlayerActivityChartGeometry{
		Max:    maximum,
		Points: make([][3]float64, len(points)),
	}
	for index, point := range points {
		geometry.Points[index] = [3]float64{
			chartCoordinate(index, len(points)),
			float64(positiveInt(point.Games)) / maximum,
			float64(positiveInt(point.Wins)) / maximum,
		}
	}
	return geometry
}

func RecentActivity(points []model.UserActivityPoint) []model.UserActivityPoint {
	if len(points) <= RecentActivityLimit {
		return points
	}
	return points[len(points)-RecentActivityLimit:]
}

func positiveInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func chartCoordinate(index, count int) float64 {
	if count <= 1 {
		return 0.5
	}
	return float64(index) / float64(count-1)
}
