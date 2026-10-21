package dev.jaderss.primebench.spring;

import dev.jaderss.primebench.spring.Downstream.Account;
import dev.jaderss.primebench.spring.Downstream.Rates;
import dev.jaderss.primebench.spring.Downstream.TransactionList;
import java.net.http.HttpClient;
import org.springframework.aot.hint.annotation.RegisterReflectionForBinding;
import org.springframework.http.client.JdkClientHttpRequestFactory;
import org.springframework.stereotype.Component;
import org.springframework.web.client.RestClient;

@Component
@RegisterReflectionForBinding({Account.class, TransactionList.class, Rates.class})
class DownstreamClient {

    private final RestClient restClient;

    DownstreamClient(RestClient.Builder builder, DownstreamProperties properties) {
        HttpClient httpClient = HttpClient.newBuilder()
                .version(HttpClient.Version.HTTP_1_1)
                .connectTimeout(properties.timeout())
                .build();
        JdkClientHttpRequestFactory requestFactory = new JdkClientHttpRequestFactory(httpClient);
        requestFactory.setReadTimeout(properties.timeout());
        this.restClient =
                builder.baseUrl(properties.url()).requestFactory(requestFactory).build();
    }

    Account account(String accountId) {
        return restClient.get().uri("/accounts/{id}", accountId).retrieve().body(Account.class);
    }

    TransactionList transactions(String accountId) {
        return restClient
                .get()
                .uri("/accounts/{id}/transactions", accountId)
                .retrieve()
                .body(TransactionList.class);
    }

    Rates rates() {
        return restClient.get().uri("/rates").retrieve().body(Rates.class);
    }
}
