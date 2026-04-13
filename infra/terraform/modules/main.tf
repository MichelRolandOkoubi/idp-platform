variable "cluster_name"     { type = string }
variable "cluster_version"  { type = string; default = "1.29" }
variable "vpc_id"           { type = string }
variable "subnet_ids"       { type = list(string) }
variable "environment"      { type = string }

# ── EKS Cluster ───────────────────────────────────────────────────────────────
resource "aws_eks_cluster" "main" {
  name     = var.cluster_name
  role_arn = aws_iam_role.cluster.arn
  version  = var.cluster_version

  vpc_config {
    subnet_ids              = var.subnet_ids
    endpoint_private_access = true
    endpoint_public_access  = true
    security_group_ids      = [aws_security_group.cluster.id]
  }

  enabled_cluster_log_types = ["api", "audit", "authenticator"]

  encryption_config {
    resources = ["secrets"]
    provider {
      key_arn = aws_kms_key.eks.arn
    }
  }

  depends_on = [
    aws_iam_role_policy_attachment.cluster_policy,
    aws_cloudwatch_log_group.eks,
  ]
}

# ── KMS for encryption ────────────────────────────────────────────────────────
resource "aws_kms_key" "eks" {
  description             = "EKS cluster encryption key"
  deletion_window_in_days = 7
  enable_key_rotation     = true
}

# ── CloudWatch log group ──────────────────────────────────────────────────────
resource "aws_cloudwatch_log_group" "eks" {
  name              = "/aws/eks/${var.cluster_name}/cluster"
  retention_in_days = 30
}

# ── Node Groups ───────────────────────────────────────────────────────────────
resource "aws_eks_node_group" "system" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "system"
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = var.subnet_ids
  instance_types  = ["t3.medium"]
  ami_type        = "AL2_x86_64"
  disk_size       = 50

  scaling_config {
    desired_size = 2
    min_size     = 2
    max_size     = 4
  }

  update_config { max_unavailable = 1 }

  labels = {
    "node-role" = "system"
  }

  taint {
    key    = "CriticalAddonsOnly"
    value  = "true"
    effect = "NO_SCHEDULE"
  }
}

resource "aws_eks_node_group" "workers" {
  cluster_name    = aws_eks_cluster.main.name
  node_group_name = "workers"
  node_role_arn   = aws_iam_role.node.arn
  subnet_ids      = var.subnet_ids
  instance_types  = ["m5.large", "m5a.large", "m5d.large"]
  ami_type        = "AL2_x86_64"

  scaling_config {
    desired_size = 2
    min_size     = 1
    max_size     = 10
  }

  update_config { max_unavailable_percentage = 25 }

  labels = {
    "node-role"   = "worker"
    "environment" = var.environment
  }

  tags = {
    "k8s.io/cluster-autoscaler/enabled"                        = "true"
    "k8s.io/cluster-autoscaler/${var.cluster_name}"            = "owned"
  }
}

# ── IAM ───────────────────────────────────────────────────────────────────────
resource "aws_iam_role" "cluster" {
  name = "${var.cluster_name}-cluster-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "eks.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "cluster_policy" {
  policy_arn = "arn:aws:iam::aws:policy/AmazonEKSClusterPolicy"
  role       = aws_iam_role.cluster.name
}

resource "aws_iam_role" "node" {
  name = "${var.cluster_name}-node-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect    = "Allow"
      Principal = { Service = "ec2.amazonaws.com" }
      Action    = "sts:AssumeRole"
    }]
  })
}

resource "aws_iam_role_policy_attachment" "node_policies" {
  for_each = toset([
    "arn:aws:iam::aws:policy/AmazonEKSWorkerNodePolicy",
    "arn:aws:iam::aws:policy/AmazonEKS_CNI_Policy",
    "arn:aws:iam::aws:policy/AmazonEC2ContainerRegistryReadOnly",
    "arn:aws:iam::aws:policy/CloudWatchAgentServerPolicy",
  ])
  policy_arn = each.value
  role       = aws_iam_role.node.name
}

# ── Security Groups ───────────────────────────────────────────────────────────
resource "aws_security_group" "cluster" {
  name        = "${var.cluster_name}-cluster-sg"
  description = "EKS cluster security group"
  vpc_id      = var.vpc_id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# ── Outputs ───────────────────────────────────────────────────────────────────
output "cluster_endpoint"    { value = aws_eks_cluster.main.endpoint }
output "cluster_name"        { value = aws_eks_cluster.main.name }
output "cluster_ca_data"     { value = aws_eks_cluster.main.certificate_authority[0].data }
output "node_role_arn"       { value = aws_iam_role.node.arn }