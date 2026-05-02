# ACM certificate for storage.xanderbilla.com (must be in us-east-1 for CloudFront)
resource "aws_acm_certificate" "storage" {
  domain_name       = var.storage_domain_name
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }

  tags = local.common_tags
}

resource "aws_route53_record" "storage_cert_validation" {
  for_each = {
    for dvo in aws_acm_certificate.storage.domain_validation_options : dvo.domain_name => {
      name   = dvo.resource_record_name
      record = dvo.resource_record_value
      type   = dvo.resource_record_type
    }
  }

  zone_id = var.route53_zone_id
  name    = each.value.name
  type    = each.value.type
  ttl     = 60
  records = [each.value.record]
}

resource "aws_acm_certificate_validation" "storage" {
  certificate_arn         = aws_acm_certificate.storage.arn
  validation_record_fqdns = [for record in aws_route53_record.storage_cert_validation : record.fqdn]
}

# CloudFront distribution for storage subdomain
resource "aws_cloudfront_distribution" "storage" {
  enabled             = true
  comment             = "${var.project_name}-${var.environment} storage CDN"
  default_root_object = ""
  price_class         = "PriceClass_100"

  aliases = [var.storage_domain_name]

  origin {
    domain_name = module.s3.bucket_regional_domain_name
    origin_id   = "s3-${module.s3.bucket_name}"

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    allowed_methods        = ["GET", "HEAD", "OPTIONS"]
    cached_methods         = ["GET", "HEAD"]
    target_origin_id       = "s3-${module.s3.bucket_name}"
    viewer_protocol_policy = "redirect-to-https"
    compress               = true

    forwarded_values {
      query_string = false
      headers      = ["Origin", "Access-Control-Request-Headers", "Access-Control-Request-Method"]

      cookies {
        forward = "none"
      }
    }

    min_ttl     = 0
    default_ttl = 86400
    max_ttl     = 31536000
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    acm_certificate_arn      = aws_acm_certificate_validation.storage.certificate_arn
    ssl_support_method       = "sni-only"
    minimum_protocol_version = "TLSv1.2_2021"
  }

  tags = local.common_tags
}

# Route53 ALIAS record for storage subdomain → CloudFront
resource "aws_route53_record" "storage" {
  zone_id = var.route53_zone_id
  name    = var.storage_domain_name
  type    = "A"

  alias {
    name                   = aws_cloudfront_distribution.storage.domain_name
    zone_id                = aws_cloudfront_distribution.storage.hosted_zone_id
    evaluate_target_health = false
  }
}
