FROM php:8.3-cli-alpine

WORKDIR /var/www

COPY src ./src
COPY public ./public

RUN addgroup -S adminapp && adduser -S -G adminapp -u 10001 adminapp \
    && chown -R adminapp:adminapp /var/www

USER adminapp

ENV PORT=8081

EXPOSE 8081

CMD ["sh", "-c", "php -S 0.0.0.0:${PORT} -t public public/router.php"]
