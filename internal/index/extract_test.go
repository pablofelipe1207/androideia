package index

import (
	"fmt"
	"testing"
)

func TestExtractRealCounterViewModel(t *testing.T) {
	extractor := NewTreeSitterExtractor()
	content := `package com.example.mviexample

import com.example.mviexample.ui.CounterEffect
import com.example.mviexample.ui.CounterEvent
import com.example.mviexample.ui.CounterState
import com.example.mviexample.ui.base.BaseViewModel


class CounterViewModel : BaseViewModel<CounterState,CounterEffect, CounterEvent>() {

    override fun createInitialState() = CounterState(0)
    override fun handleEvent(event: CounterEvent) {
        when (event) {
            is CounterEvent.Increment -> {
                setState { copy(count = uiState.value.count + 1) }
                validateNumber()
            }

            is CounterEvent.Decrement -> {
                setState { copy(count = uiState.value.count - 1) }
            }
        }
    }

    private fun validateNumber(value : Int = 100){
        if(uiState.value.count == value){
            setEffect { CounterEffect.ShowMessage("Numero : $value") }
        }
    }

}
`
	symbols := extractor.ExtractSymbols("CounterViewModel.kt", content)
	fmt.Printf("Total symbols: %d\n", len(symbols))
	for _, s := range symbols {
		fmt.Printf("Name: %s, Kind: %s\n", s.Name, s.Kind)
	}
	
	feature := extractor.ExtractFeature(symbols)
	fmt.Printf("Extracted feature: '%s'\n", feature)
}
