package com.example.testapp.domain.usecase

import com.example.testapp.data.repository.UserRepository

class LoginUseCase(private val userRepository: UserRepository) {
    suspend operator fun invoke(username: String, password: String): Boolean {
        return userRepository.login(username, password)
    }
}
