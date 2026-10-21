ARG SPRING_JAVA_VERSION=25

FROM eclipse-temurin:${SPRING_JAVA_VERSION}-jdk AS build
WORKDIR /src
COPY .mvn .mvn
COPY mvnw pom.xml ./
RUN ./mvnw -q -B dependency:go-offline
COPY src src
RUN ./mvnw -q -B package && cp target/prime-bench-spring-*.jar /app.jar

FROM eclipse-temurin:${SPRING_JAVA_VERSION}-jre AS jvm
COPY --from=build /app.jar /app/app.jar
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["sh", "-c", "exec java $JAVA_OPTS -jar /app/app.jar"]

FROM ghcr.io/graalvm/native-image-community:${SPRING_JAVA_VERSION} AS native-build
WORKDIR /src
COPY .mvn .mvn
COPY mvnw pom.xml ./
RUN ./mvnw -q -B dependency:go-offline
COPY src src
RUN ./mvnw -q -B -Pnative -DskipTests native:compile && cp target/prime-bench-spring /prime-bench-spring

FROM gcr.io/distroless/base-debian12:nonroot AS native
COPY --from=native-build /prime-bench-spring /prime-bench-spring
EXPOSE 8080
ENTRYPOINT ["/prime-bench-spring"]
