package com.example.testapp.ui.login

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import kotlinx.coroutines.launch

class LoginViewModel : ViewModel() {
    fun login(username: String, password: String) {
        viewModelScope.launch {
            // Login logic
        }
    }
}
