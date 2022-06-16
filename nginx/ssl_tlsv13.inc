#ssl_ecdh_curve X25519:P-256:P-384:P-224:P-521;
ssl_prefer_server_ciphers on;
ssl_protocols TLSv1 TLSv1.1 TLSv1.2 TLSv1.3;
#ssl_protocols TLSv1.3;
ssl_ciphers 'ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-ECDSA-CHACHA20-POLY1305:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-CHACHA20-POLY1305:ECDHE+AES128:RSA+AES128:ECDHE+AES256:RSA+AES256:ECDHE+3DES:RSA+3DES';
#more_set_headers "Strict-Transport-Security: 'max-age=31536000; includeSubDomains'" always;
more_clear_headers "Strict-Transport-Security";
