use hyper::{Body, Request, Response, Server};
use hyper::service::{make_service_fn, service_fn};
use serde::{Deserialize};
use std::convert::Infallible;
use std::net::SocketAddr;
use serde_json::Value;
use std::alloc::{alloc, dealloc, Layout};
use std::ptr;
use std::time::{Duration, Instant};
use rsa::PrivateKeyEncoding;
use rsa::{PublicKeyEncoding, RSAPrivateKey, RSAPublicKey};
use base64::encode;
use rand_core::OsRng;
use serde::Serialize;

#[derive(Debug, Serialize)]
struct FuncResponse {
    private_key: String,
    public_key: String,
}

fn rsa_keygen() -> FuncResponse {
    let private_key = RSAPrivateKey::new(&mut OsRng, 2048).expect("failed to generate a key");
    let public_key = RSAPublicKey::from(&private_key);

    FuncResponse {
        private_key: encode(private_key.to_pkcs8().unwrap()),
        public_key: encode(public_key.to_pkcs8().unwrap()),
    }
}

async fn handle(req: Request<Body>) -> Result<Response<Body>, Infallible> {
    let start = Instant::now();
    
    // Only accept POST requests
    if req.method() != hyper::Method::POST {
        return Ok(Response::new(Body::from("Only POST method is supported")));
    }

    let fun_res = rsa_keygen();

    let concatenated_keys = format!("{}:{}",
        fun_res.private_key, fun_res.public_key);

    println!(
        "RSA Executed\n"
    );

    Ok(Response::new(Body::from(concatenated_keys)))
}

#[tokio::main]
async fn _main() {
    let fun_res = rsa_keygen();

    let concatenated_keys = format!("{}:{}",
        fun_res.private_key, fun_res.public_key);

    println!(
        "RSA Executed\n"
    );
}

#[tokio::main]
async fn main() {
    let fun_res = rsa_keygen();

    let concatenated_keys = format!("{}:{}",
        fun_res.private_key, fun_res.public_key);

    println!(
        "RSA Executed\n"
    );
}
