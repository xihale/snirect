# 保留 Compose 相关元数据
-keepclassmembers class androidx.compose.runtime.Recomposer { *; }

# kotlinx.serialization
-keepattributes *Annotation*, InnerClasses
-keepclassmembers class ** {
    @kotlinx.serialization.Serializable *;
}
-keepclassmembers class ** {
    kotlinx.serialization.KSerializer serializer(...);
}

# ktoml (TOML parsing)
-keep class com.akuleshov7.ktoml.** { *; }

# 忽略常见警告
-dontwarn androidx.compose.**
-dontwarn kotlinx.serialization.**
