export const Logo = (props: React.SVGProps<SVGSVGElement>) => {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="350"
      height="350"
      viewBox="0 0 350 350"
      fill="none"
      {...props}
    >
      <rect width="350" height="350" fill="black" />
      <path
        d="M25.2336 175.467L100.031 45.4467L250.031 45.213L325.233 175L250.436 305.02L100.436 305.254L25.2336 175.467Z"
        fill="white"
      />
    </svg>
  );
};
