package dev.jaderss.primebench.spring;

import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import java.time.Duration;
import org.springframework.boot.context.properties.ConfigurationProperties;
import org.springframework.validation.annotation.Validated;

@Validated
@ConfigurationProperties("downstream")
record DownstreamProperties(@NotBlank String url, @NotNull Duration timeout) {}
