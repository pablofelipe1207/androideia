package com.example.testapp.data.repository

interface UserRepository {
    suspend fun login(username: String, password: String): Boolean
    suspend fun getUser(id: String): User?
}

class UserRepositoryImpl : UserRepository {
    override suspend fun login(username: String, password: String): Boolean {
        // Implementation
        return true
    }

    override suspend fun getUser(id: String): User? {
        // Implementation
        return null
    }
}

data class User(
    val id: String,
    val name: String,
    val email: String
)

data class Product(
    val id: String,
    val name: String,
    val price: Double
)