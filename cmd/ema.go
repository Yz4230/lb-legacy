package cmd

// EMA構造体
type EMA struct {
	alpha float64
	value float64
	init  bool
}

// NewEMAは新しいEMA計算用の構造体を作成します
func NewEMA(span int) *EMA {
	if span <= 0 {
		panic("Span must be greater than 0")
	}
	alpha := 2.0 / float64(span+1)
	return &EMA{alpha: alpha, init: false}
}

// Updateは新しい値を受け取りEMAを更新します
func (e *EMA) Update(newValue float64) float64 {
	if !e.init {
		// 初回は新しい値をそのままEMA値に設定
		e.value = newValue
		e.init = true
	} else {
		// EMA計算
		e.value = e.alpha*newValue + (1-e.alpha)*e.value
	}
	return e.value
}

// GetValueは現在のEMA値を取得します
func (e *EMA) GetValue() float64 {
	if !e.init {
		panic("EMA has not been initialized yet")
	}
	return e.value
}
