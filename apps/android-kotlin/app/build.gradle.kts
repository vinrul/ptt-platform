import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("org.jetbrains.kotlin.plugin.compose")
    id("com.google.gms.google-services")
}

val localProperties = Properties().apply {
    val propertiesFile = rootProject.file("local.properties")
    if (propertiesFile.exists()) {
        propertiesFile.inputStream().use(::load)
    }
}

fun signingValue(environmentName: String, propertyName: String): String? =
    System.getenv(environmentName)
        ?.takeIf { it.isNotBlank() }
        ?: localProperties.getProperty(propertyName)?.takeIf { it.isNotBlank() }

val releaseStoreFile = signingValue("ANDROID_RELEASE_STORE_FILE", "release.storeFile")
val releaseStorePassword = signingValue("ANDROID_RELEASE_STORE_PASSWORD", "release.storePassword")
val releaseKeyAlias = signingValue("ANDROID_RELEASE_KEY_ALIAS", "release.keyAlias")
val releaseKeyPassword = signingValue("ANDROID_RELEASE_KEY_PASSWORD", "release.keyPassword")

android {
    namespace = "id.nuwiarul.pttfleet"
    compileSdk = 35

    defaultConfig {
        applicationId = "id.nuwiarul.pttfleet"
        minSdk = 26
        targetSdk = 35
        versionCode = 1
        versionName = "0.1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
        manifestPlaceholders["usesCleartextTraffic"] = "false"
    }

    signingConfigs {
        if (
            releaseStoreFile != null &&
            releaseStorePassword != null &&
            releaseKeyAlias != null &&
            releaseKeyPassword != null
        ) {
            create("release") {
                storeFile = file(releaseStoreFile)
                storePassword = releaseStorePassword
                keyAlias = releaseKeyAlias
                keyPassword = releaseKeyPassword
            }
        }
    }

    buildTypes {
        debug {
            manifestPlaceholders["usesCleartextTraffic"] = "true"
        }
        release {
            isMinifyEnabled = false
            manifestPlaceholders["usesCleartextTraffic"] = "false"
            signingConfig = signingConfigs.findByName("release")
            proguardFiles(
                getDefaultProguardFile("proguard-android-optimize.txt"),
                "proguard-rules.pro",
            )
        }
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    kotlinOptions {
        jvmTarget = "17"
    }

    buildFeatures {
        viewBinding = true
        buildConfig = true
        compose = true
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.16.0")
    implementation("androidx.appcompat:appcompat:1.6.1")
    implementation("androidx.lifecycle:lifecycle-runtime-ktx:2.9.2")
    implementation(platform("androidx.compose:compose-bom:2025.07.00"))
    implementation("androidx.compose.material3:material3")
    implementation("androidx.compose.material:material-icons-extended")
    implementation("androidx.compose.ui:ui")
    implementation("androidx.compose.ui:ui-tooling-preview")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-android:1.9.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("com.google.android.gms:play-services-location:21.3.0")
    implementation("io.github.jaredmdobson:concentus:1.0.2")
    implementation("org.maplibre.gl:android-sdk:13.3.0")

    // Firebase
    implementation(platform("com.google.firebase:firebase-bom:33.1.0"))
    implementation("com.google.firebase:firebase-messaging-ktx")

    testImplementation("junit:junit:4.13.2")
    testImplementation("org.json:json:20240303")
    debugImplementation("androidx.compose.ui:ui-tooling")
}
