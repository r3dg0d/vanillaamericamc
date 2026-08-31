plugins {
    java
    id("com.gradleup.shadow") version "9.2.2"
}

group = "dev.vanillaamericaplus"
version = "1.0.0"

repositories {
    mavenCentral()
    maven("https://repo.papermc.io/repository/maven-public/") {
        name = "papermc"
    }
    maven("https://repo.opencollab.dev/main/") {
        name = "opencollab"
    }
}

dependencies {
    compileOnly("io.papermc.paper:paper-api:26.2.build.121-stable")
    compileOnly("org.geysermc.floodgate:api:2.2.5-SNAPSHOT")
    compileOnly("net.luckperms:api:5.5")

    implementation("org.xerial:sqlite-jdbc:3.50.3.0")

    testImplementation(platform("org.junit:junit-bom:5.13.4"))
    testImplementation("org.junit.jupiter:junit-jupiter")
    testImplementation("org.assertj:assertj-core:3.27.4")
    testRuntimeOnly("org.junit.platform:junit-platform-launcher")
}

java {
    toolchain.languageVersion.set(JavaLanguageVersion.of(25))
    withSourcesJar()
}

val pluginVersion = version.toString()

tasks {
    processResources {
        inputs.property("pluginVersion", pluginVersion)
        filesMatching("plugin.yml") {
            expand("version" to pluginVersion)
        }
    }

    test {
        useJUnitPlatform()
    }

    shadowJar {
        archiveClassifier.set("")
        relocate("org.sqlite", "dev.vanillaamericaplus.libs.sqlite")
        minimize {
            exclude(dependency("org.xerial:sqlite-jdbc:.*"))
        }
    }

    jar {
        enabled = false
    }

    build {
        dependsOn(shadowJar)
    }
}

