package dev.jaderss.primebench.spring;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.ConfigurationPropertiesScan;

@SpringBootApplication
@ConfigurationPropertiesScan
public class PrimeBenchSpringApplication {

    public static void main(String[] args) {
        SpringApplication.run(PrimeBenchSpringApplication.class, args);
    }
}
