package dev.jaderss.primebench.spring;

import dev.jaderss.primebench.spring.AggregateService.Aggregate;
import jakarta.validation.constraints.Pattern;
import java.util.concurrent.CompletionException;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.http.HttpStatus;
import org.springframework.http.ProblemDetail;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;
import org.springframework.web.client.RestClientResponseException;

@RestController
class GatewayController {

    private static final Logger LOG = LoggerFactory.getLogger(GatewayController.class);

    private final AggregateService aggregateService;

    GatewayController(AggregateService aggregateService) {
        this.aggregateService = aggregateService;
    }

    @GetMapping("/aggregate")
    Aggregate aggregate(@RequestParam @Pattern(regexp = "[A-Za-z0-9-]{1,64}") String account) {
        return aggregateService.aggregate(account);
    }

    @ExceptionHandler(CompletionException.class)
    ProblemDetail downstreamFailed(CompletionException exception) {
        LOG.warn("downstream call failed: {}", describe(exception.getCause()));
        return ProblemDetail.forStatusAndDetail(HttpStatus.BAD_GATEWAY, "downstream unavailable");
    }

    private static String describe(Throwable cause) {
        return cause instanceof RestClientResponseException response
                ? "downstream answered " + response.getStatusCode().value()
                : cause.getClass().getSimpleName();
    }
}
