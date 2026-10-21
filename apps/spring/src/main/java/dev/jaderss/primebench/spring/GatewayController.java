package dev.jaderss.primebench.spring;

import dev.jaderss.primebench.spring.AggregateService.Aggregate;
import jakarta.validation.constraints.Pattern;
import java.util.concurrent.CompletionException;
import org.springframework.http.HttpStatus;
import org.springframework.http.ProblemDetail;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

@RestController
class GatewayController {

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
        return ProblemDetail.forStatusAndDetail(
                HttpStatus.BAD_GATEWAY, String.valueOf(exception.getCause().getMessage()));
    }
}
