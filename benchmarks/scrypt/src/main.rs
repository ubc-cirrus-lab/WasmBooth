use hyper::{Body, Request, Response, Server};
use hyper::service::{make_service_fn, service_fn};
use serde::{Deserialize};
use std::convert::Infallible;
use std::net::SocketAddr;
use serde_json::Value;
use std::alloc::{alloc, dealloc, Layout};
use std::ptr;
use std::time::{Duration, Instant};
use rand_core::OsRng;
use serde::Serialize;
use rand::distributions::Alphanumeric;
use rand::Rng;
use scrypt::scrypt;
use scrypt::password_hash::Output;
use rand::prelude::*;

#[derive(Debug, Deserialize)]
struct FuncInput {
    input_vec: Vec<String>,
}

#[derive(Debug, Serialize)]
struct FuncResponse {
    hashed_results: Vec<String>,
}

#[inline(never)]
fn hash_password(input: FuncInput) -> FuncResponse {
    // Litecoin scrypt parameters: N=1024 (this lib takes log2(N)), p = 1, r = 1
    let mut results = vec![];
    for val in input.input_vec {
        let params = scrypt::Params::new(10, 1, 1).unwrap();
        //let salt = Salt::new(&*SALT.as_ref()).unwrap();
        // Litecoin uses the same input bytes as the salt value
        // https://litecoin.info/index.php/Scrypt
        //let salt = Salt::new(&val).unwrap();
        let mut output = [0u8; 32];
        //let hash = Scrypt.hash_password_customized(val.as_bytes(), Some(ALG_ID), None, params, salt).unwrap();
        scrypt(val.as_bytes(), val.as_bytes(), &params, &mut output).unwrap();
        let output_fmt = Output::new(&output).unwrap();
        results.push(output_fmt.to_string());
    }
    FuncResponse {
        hashed_results: results,
    }
}

fn create_large_input(seed: u64, num_strings: usize, string_size: usize) -> FuncInput {
    let mut rng = StdRng::seed_from_u64(seed); // Using a deterministic RNG

    // Create the vector of random strings
    let input_vec: Vec<String> = (0..num_strings)
        .map(|_| generate_random_string(&mut rng, string_size))
        .collect();

    FuncInput { input_vec }
}

// Function to generate a random string of a given size using Alphanumeric characters
fn generate_random_string(rng: &mut StdRng, size: usize) -> String {
    rng.sample_iter(&Alphanumeric)
        .take(size)
        .map(char::from)
        .collect()
}

async fn handle(req: Request<Body>) -> Result<Response<Body>, Infallible> {
    let start = Instant::now();
    
    // Only accept POST requests
    if req.method() != hyper::Method::POST {
        return Ok(Response::new(Body::from("Only POST method is supported")));
    }

    let input_data = create_large_input(42, 1000, 64);

    let scrypt_pass = hash_password(input_data);

    let first_five_concat: String = scrypt_pass.hashed_results
        .iter()
        .take(5)
        .cloned() // Create a copy of each element
        .collect::<Vec<String>>()
        .join(""); // Join into a single string

    println!(
        "SCRYPT Executed\n"
    );

    Ok(Response::new(Body::from(first_five_concat)))
}

#[tokio::main]
async fn _main() {
    let input_data = create_large_input(42, 1000, 64);

    let scrypt_pass = hash_password(input_data);

    let first_five_concat: String = scrypt_pass.hashed_results
        .iter()
        .take(5)
        .cloned() // Create a copy of each element
        .collect::<Vec<String>>()
        .join(""); // Join into a single string

    println!(
        "SCRYPT Executed\n"
    );
}

#[tokio::main]
async fn main() {
    let input_data = create_large_input(42, 1000, 64);

    let scrypt_pass = hash_password(input_data);

    let first_five_concat: String = scrypt_pass.hashed_results
        .iter()
        .take(5)
        .cloned() // Create a copy of each element
        .collect::<Vec<String>>()
        .join(""); // Join into a single string

    println!(
        "SCRYPT Executed\n"
    );
}
