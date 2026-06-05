package com.example.testapp.di

import dagger.Module
import dagger.hilt.InstallIn
import dagger.hilt.components.SingletonComponent
import com.example.testapp.data.repository.UserRepository
import com.example.testapp.data.repository.UserRepositoryImpl
import dagger.Provides

@Module
@InstallIn(SingletonComponent::class)
class RepositoryModule {
    @Provides
    fun provideUserRepository(): UserRepository {
        return UserRepositoryImpl()
    }
}
