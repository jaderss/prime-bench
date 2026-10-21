ARG JAVA_VERSION=27
FROM eclipse-temurin:${JAVA_VERSION}-jdk AS build
ARG JAVA_VERSION
WORKDIR /src
COPY .mvn .mvn
COPY mvnw pom.xml ./
RUN ./mvnw -q -B -Djava.release=${JAVA_VERSION} dependency:go-offline
COPY src src
RUN ./mvnw -q -B -Djava.release=${JAVA_VERSION} package

FROM eclipse-temurin:${JAVA_VERSION}-jre
COPY --from=build /src/target/prime-bench.jar /app/prime-bench.jar
USER 65532:65532
ENV RESULTS_DIR=/results
EXPOSE 8080
ENTRYPOINT ["sh", "-c", "exec java $JAVA_OPTS -jar /app/prime-bench.jar"]
